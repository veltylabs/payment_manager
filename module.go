package paymentmanager

import (
	"sync"

	"webtyp.com/events"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/money"
	"webtyp.com/orm"
	"webtyp.com/time"
)

// Eventos publicados después de cada mutación exitosa. Payload: *Payment, o
// *Refund en EventPaymentRefunded.
const (
	EventPaymentPaid     = "payment.paid"
	EventPaymentFailed   = "payment.failed"
	EventPaymentRefunded = "payment.refunded"
)

// Deps son las dependencias que inyecta la raíz de composición.
type Deps struct {
	IDs       model.IDGenerator
	Timeouts  Timeouts
	TenantID  string           // requerido: el tenant al que sirve esta instancia
	Publisher events.Publisher // opcional — nil desactiva la publicación
	Gateway   Gateway          // opcional — nil: el canal online responde ErrGatewayUnavailable
	Charges   []ChargeSource   // requerido: quién responde el monto de cada módulo de referencia
	Terminal  Terminal         // opcional — nil: el cobro con máquina responde ErrTerminalUnavailable
	BankDays  BankDays         // requerido si el Terminal o el Gateway implementan PayoutTerms
}

// Module registra, confirma y devuelve pagos.
type Module struct {
	timeouts  Timeouts
	store     *orm.Store
	db        *orm.DB
	repo      *repository
	pub       events.Publisher
	gw        Gateway
	retURL    string
	charges   []ChargeSource
	term      Terminal
	receiptMu sync.Mutex
	bank      BankDays
	tenantID  string
}

// New conecta el módulo a un *orm.DB ya migrado (ver el subpaquete migrate).
func New(db *orm.DB, deps Deps) (*Module, error) {
	if deps.IDs == nil {
		return nil, fmt.Err("payment_manager: Deps.IDs is required")
	}
	t, err := deps.Timeouts.withDefaults()
	if err != nil {
		return nil, err
	}
	if len(deps.Charges) == 0 {
		return nil, fmt.Err("payment_manager: Deps.Charges is required")
	}

	for _, c := range deps.Charges {
		if c.Device != "" && deps.Terminal == nil {
			return nil, fmt.Err("payment_manager: Deps.Terminal is required when a ChargeSource names a Device")
		}
	}

	_, termHasPayout := deps.Terminal.(PayoutTerms)
	_, gwHasPayout := deps.Gateway.(PayoutTerms)
	if (termHasPayout || gwHasPayout) && deps.BankDays == nil {
		return nil, fmt.Err("payment_manager: Deps.BankDays is required when an adapter implements PayoutTerms")
	}
	return &Module{
		db:       db,
		store:    orm.TenantStore(db, deps.TenantID, deps.IDs),
		timeouts: t,
		repo:     &repository{db: db},
		pub:      deps.Publisher,
		gw:       deps.Gateway,
		charges:  deps.Charges,
		term:     deps.Terminal,
		bank:     deps.BankDays,
		tenantID: deps.TenantID,
	}, nil
}

func (m *Module) publish(topic string, payload model.Model) {
	if m.pub != nil {
		m.pub.Publish(events.Event{Topic: topic, Payload: payload})
	}
}

func isCounterMethod(method string) bool {
	return method == MethodCash || method == MethodDebit || method == MethodCredit || method == MethodTransfer
}

func (m *Module) source(referenceModule string) (ChargeSource, bool) {
	for _, c := range m.charges {
		if c.Module == referenceModule {
			return c, true
		}
	}
	return ChargeSource{}, false
}

func (m *Module) hasTerminal(referenceModule string) bool {
	src, ok := m.source(referenceModule)
	return m.term != nil && ok && src.Device != ""
}

// Charge pregunta al módulo dueño de la referencia cuánto se cobra. El monto
// nunca viene del cliente.
func (m *Module) Charge(tenantId, referenceModule, referenceId string) (amount int64, currency string, payable bool, err error) {
	src, ok := m.source(referenceModule)
	if !ok {
		return 0, "", false, ErrUnknownReference
	}
	amount, currency, payable, err = src.Reader.Charge(tenantId, referenceId)
	if err != nil {
		return 0, "", false, err
	}
	if currency != CurrencyCLP && currency != CurrencyUSD {
		return 0, "", false, ErrUnsupportedCurrency
	}
	return amount, currency, payable, nil
}

// Due devuelve lo que falta cobrar de una referencia: el monto que informa su ChargeReader menos el
// bruto de los pagos PAID y PARTIALLY_REFUNDED de esa referencia. Un pago PENDING, FAILED, CANCELLED,
// EXPIRED o REFUNDED no cuenta. El monto nunca viene del cliente.
func (m *Module) Due(tenantId, referenceModule, referenceId string) (amount int64, currency string, err error) {
	payments, err := m.repo.paymentsByReference(tenantId, referenceModule, referenceId)
	if err != nil {
		return 0, "", err
	}
	var covered int64
	for _, p := range payments {
		if p.Status == StatusPaid || p.Status == StatusPartiallyRefunded {
			covered += p.Amount
		}
	}
	total, currency, payable, err := m.Charge(tenantId, referenceModule, referenceId)
	if err != nil {
		return 0, "", err
	}
	if !payable {
		if covered > 0 {
			return 0, "", ErrAlreadyPaid
		}
		return 0, "", ErrNotPayable
	}
	if total-covered <= 0 {
		return 0, "", ErrAlreadyPaid
	}
	return total - covered, currency, nil
}

// CreateCounterPayment registra un pago hecho en el mesón. Nace pagado.
// Repetir el mismo id con la misma referencia devuelve el pago guardado.
func (m *Module) settleTerminal(p Payment, res TerminalResult, fromReconcile bool) (Payment, error) {
	switch res.Outcome {
	case OutcomeAuthorized:
		if res.Amount != p.Amount {
			failed, err := m.close(p, EventFail)
			if err != nil {
				return failed, err
			}
			return failed, ErrAmountMismatch
		}
		p.AuthorizationCode = res.AuthorizationCode
		p.CardLast4 = res.CardLast4
		p.PaidAt = time.NowUnix()
		return m.close(p, EventPay)
	case OutcomeRejected:
		return m.close(p, EventFail)
	case OutcomeCancelled:
		return m.close(p, EventCancel)
	}
	if fromReconcile {
		return m.close(p, EventExpire)
	}
	return p, nil
}

// CompleteOnlinePayment cierra un pago online cuando el cliente vuelve de la pasarela. La vuelta solo IDENTIFICA el pago
// (OnlineReturn no trae resultado): lo que pasó lo pregunta el módulo a la pasarela con el token guardado, nunca con el
// de la petición, así que nadie puede cancelar ni vencer un pago ajeno con una dirección armada a mano. Un pago que ya no
// está pendiente se devuelve igual (idempotente); un resultado aún pendiente lo deja pendiente (la conciliación lo vence).
//
// Ejemplo: p, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{PaymentId: "p1", Token: tokenDeLaVuelta})
func (m *Module) CompleteOnlinePayment(ret OnlineReturn) (Payment, error) {
	if m.gw == nil {
		return Payment{}, ErrGatewayUnavailable
	}
	if ret.PaymentId == "" && ret.Token == "" {
		return Payment{}, ErrMissingArgs
	}
	var p Payment
	var err error
	if ret.PaymentId != "" {
		p, err = m.repo.paymentById(m.tenantID, ret.PaymentId)
	} else {
		p, err = m.repo.paymentByProviderToken(m.tenantID, ret.Token)
	}
	if err != nil {
		return Payment{}, err
	}
	if p.Status != StatusPending {
		return p, nil
	}
	if ret.Token != "" && ret.Token != p.ProviderToken {
		return Payment{}, ErrTokenMismatch
	}
	res, err := m.gw.Confirm(p.ProviderToken)
	if err != nil {
		return p, ErrGatewayFailed
	}
	return m.settle(p, res, false)
}

// settle asienta un resultado de la pasarela (diagrama §6). fromReconcile
// decide qué pasa con un resultado aún pendiente: la conciliación lo vence.
func (m *Module) settle(p Payment, result GatewayResult, fromReconcile bool) (Payment, error) {
	switch result.Outcome {
	case OutcomeAuthorized:
		if result.Amount != p.Amount {
			if _, err := m.gw.Refund(p.ProviderToken, result.Amount); err != nil {
				return p, ErrGatewayFailed
			}
			failed, err := m.close(p, EventFail)
			if err != nil {
				return failed, err
			}
			return failed, ErrAmountMismatch
		}
		p.AuthorizationCode = result.AuthorizationCode
		p.PaymentTypeCode = result.PaymentTypeCode
		p.Installments = result.Installments
		p.CardLast4 = result.CardLast4
		p.PaidAt = time.NowUnix()
		return m.close(p, EventPay)
	case OutcomeRejected:
		return m.close(p, EventFail)
	case OutcomeCancelled:
		return m.close(p, EventCancel)
	case OutcomeExpired:
		return m.close(p, EventExpire)
	}
	if fromReconcile {
		return m.close(p, EventExpire)
	}
	return p, nil
}

// close aplica event a un pago, lo guarda con control de revisión y publica.
func (m *Module) close(p Payment, event string) (Payment, error) {
	// close solo recibe pagos PENDING con PAY/FAIL/CANCEL/EXPIRE: la transición
	// siempre existe.
	next, _ := Transition(p.Status, event)
	expected := p.Revision
	p.Status = next
	p.Revision++
	p.UpdatedAt = time.NowUnix()
	if err := m.db.Tx(func(tx *orm.DB) error { return m.repo.updatePaymentTx(tx, &p, expected) }); err != nil {
		return Payment{}, err
	}
	switch next {
	case StatusPaid:
		if m.pub != nil {
			facts, err := m.paymentFacts(p)
			if err != nil {
				return Payment{}, err
			}
			for _, f := range facts {
				if err := money.Publish(m.pub, f); err != nil {
					return Payment{}, err
				}
			}
		}
		m.publish(EventPaymentPaid, &p)
	case StatusFailed:
		m.publish(EventPaymentFailed, &p)
	}
	return p, nil
}

// RefundPayment devuelve amount de un pago pagado. En online lo pide a la
// pasarela; en mesón registra la devolución con su medio.
func (m *Module) ReconcilePayments(tenantId string, now time.Unix) (int, error) {
	closed := 0
	if m.gw != nil {
		pending, err := m.repo.oldPendingPayments(tenantId, ChannelOnline, int64(now-PendingTimeout))
		if err != nil {
			return 0, err
		}
		for _, p := range pending {
			var err error
			if p.CreatedAt < time.Unix(now-GatewayStatusWindow) {
				_, err = m.close(p, EventExpire)
			} else {
				result, gwErr := m.gw.Status(p.ProviderToken)
				if gwErr != nil {
					continue // la pasarela no respondió: se intenta en la próxima pasada
				}
				_, err = m.settle(p, result, true)
			}
			switch err {
			case nil, ErrAmountMismatch:
				closed++
			case ErrConflict, ErrGatewayFailed:
				// otro proceso lo cerró, o la devolución falló: se reintenta después
			default:
				return closed, err
			}
		}
	}
	if m.term != nil {
		pending, err := m.repo.oldPendingPayments(tenantId, ChannelTerminal, int64(now-PendingTimeout))
		if err != nil {
			return 0, err
		}
		for _, p := range pending {
			var err error
			if p.CreatedAt < time.Unix(now-GatewayStatusWindow) {
				_, err = m.close(p, EventExpire)
			} else {
				res, termErr := m.term.Status(p.ProviderToken)
				if termErr != nil {
					continue
				}
				_, err = m.settleTerminal(p, res, true)
			}
			switch err {
			case nil, ErrAmountMismatch:
				closed++
			case ErrConflict, ErrGatewayFailed:
				// otro proceso lo cerró: se reintenta después
			default:
				return closed, err
			}
		}
	}
	return closed, nil
}

// PaidAmount verifica un pago para el módulo dueño de la referencia: devuelve
// el monto neto (pagado menos devuelto) solo si el pago es de esa referencia y
// está pagado.
func (m *Module) PaidAmount(tenantId, paymentId, referenceModule, referenceId string) (amount int64, currency string, err error) {
	p, err := m.repo.paymentById(tenantId, paymentId)
	if err != nil {
		return 0, "", err
	}
	if p.ReferenceModule != referenceModule || p.ReferenceId != referenceId {
		return 0, "", ErrNotFound
	}
	if p.Status != StatusPaid && p.Status != StatusPartiallyRefunded {
		return 0, "", ErrNotPaid
	}
	return p.Amount - p.RefundedAmount, p.Currency, nil
}

// GetPayment lee un pago del tenant.
func (m *Module) GetPayment(tenantId, id string) (Payment, error) {
	return m.repo.paymentById(tenantId, id)
}

// ListPaymentsByReference lista los pagos de una referencia.
func (m *Module) ListPaymentsByReference(tenantId, referenceModule, referenceId string) ([]Payment, error) {
	return m.repo.paymentsByReference(tenantId, referenceModule, referenceId)
}

// ListPayments lista los pagos creados en [from, to) (segundos).
func (m *Module) ListPayments(tenantId string, from, to time.Unix) ([]Payment, error) {
	return m.repo.paymentsByDateRange(tenantId, int64(from), int64(to))
}

// listPaymentsPaged lista pagos paginados, buscando opcionalmente por monto (búsqueda estricta).
func (m *Module) listPaymentsPaged(tenantId, search, after string, limit int64) ([]Payment, error) {
	return m.repo.paymentsPaged(tenantId, search, after, limit)
}

// ListRefunds lista las devoluciones de un pago.
func (m *Module) ListRefunds(tenantId, paymentId string) ([]Refund, error) {
	return m.repo.refundsByPayment(tenantId, paymentId)
}

// create valida y guarda un pago nuevo.
func (m *Module) create(p *Payment) error {
	if err := p.Validate(byte(model.ActionCreate)); err != nil {
		return err
	}
	return m.db.Create(p)
}
