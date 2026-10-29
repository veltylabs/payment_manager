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
	Publisher events.Publisher // opcional — nil desactiva la publicación
	Gateway   Gateway          // opcional — nil: el canal online responde ErrGatewayUnavailable
	Charges   []ChargeSource   // requerido: quién responde el monto de cada módulo de referencia
	Terminal  Terminal         // opcional — nil: el cobro con máquina responde ErrTerminalUnavailable
	BankDays  BankDays         // requerido si el Terminal o el Gateway implementan PayoutTerms
}

// Module registra, confirma y devuelve pagos.
type Module struct {
	db        *orm.DB
	repo      *repository
	pub       events.Publisher
	gw        Gateway
	retURL    string
	charges   []ChargeSource
	term      Terminal
	receiptMu sync.Mutex
	bank      BankDays
}

// New conecta el módulo a un *orm.DB ya migrado (ver el subpaquete migrate).
func New(db *orm.DB, deps Deps) (*Module, error) {
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
		db:      db,
		repo:    &repository{db: db},
		pub:     deps.Publisher,
		gw:      deps.Gateway,
		charges: deps.Charges,
		term:    deps.Terminal,
		bank:    deps.BankDays,
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
func (m *Module) CreateCounterPayment(cmd CreateCounterPaymentArgs) (Payment, error) {
	if cmd.TenantId == "" || cmd.Id == "" || cmd.ReferenceModule == "" || cmd.ReferenceId == "" || cmd.Method == "" {
		return Payment{}, ErrMissingArgs
	}
	existing, err := m.repo.paymentAnyTenant(cmd.Id)
	if err == nil {
		if existing.TenantId == cmd.TenantId && existing.ReferenceModule == cmd.ReferenceModule && existing.ReferenceId == cmd.ReferenceId {
			return existing, nil
		}
		return Payment{}, ErrIdTaken
	}
	if model.KindOf(err) != model.ErrorNotFound {
		return Payment{}, err
	}
	if !isCounterMethod(cmd.Method) {
		return Payment{}, ErrInvalidMethod
	}
	if (cmd.Method == MethodDebit || cmd.Method == MethodCredit) && m.hasTerminal(cmd.ReferenceModule) {
		return Payment{}, ErrTerminalRequired
	}
	amount, currency, err := m.Due(cmd.TenantId, cmd.ReferenceModule, cmd.ReferenceId)
	if err != nil {
		return Payment{}, err
	}

	receiptNumber, err := m.NextReceiptNumber(cmd.TenantId)
	if err != nil {
		return Payment{}, err
	}

	now := time.NowUnix()
	p := Payment{
		Id:              cmd.Id,
		TenantId:        cmd.TenantId,
		ReferenceModule: cmd.ReferenceModule,
		ReferenceId:     cmd.ReferenceId,
		Channel:         ChannelCounter,
		Method:          cmd.Method,
		Amount:          amount,
		Currency:        currency,
		Status:          StatusPaid,
		ReceiptNumber:   receiptNumber,
		CreatedAt:       now,
		PaidAt:          now,
		CreatedBy:       cmd.CreatedBy,
		UpdatedAt:       now,
		Revision:        1,
	}
	if err := m.create(&p); err != nil {
		return Payment{}, err
	}
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
	return p, nil
}

// StartOnlinePayment crea el pago PENDING y pide a la pasarela un token y la
// URL a la que se envía al cliente.
func (m *Module) StartOnlinePayment(cmd StartOnlinePaymentArgs) (StartOnlinePaymentResult, error) {
	if m.gw == nil {
		return StartOnlinePaymentResult{}, ErrGatewayUnavailable
	}
	if cmd.TenantId == "" || cmd.Id == "" || cmd.ReferenceModule == "" || cmd.ReferenceId == "" {
		return StartOnlinePaymentResult{}, ErrMissingArgs
	}
	if _, err := m.repo.paymentAnyTenant(cmd.Id); err == nil {
		return StartOnlinePaymentResult{}, ErrIdTaken
	} else if model.KindOf(err) != model.ErrorNotFound {
		return StartOnlinePaymentResult{}, err
	}
	amount, currency, err := m.Due(cmd.TenantId, cmd.ReferenceModule, cmd.ReferenceId)
	if err != nil {
		return StartOnlinePaymentResult{}, err
	}

	now := time.NowUnix()
	p := Payment{
		Id:              cmd.Id,
		TenantId:        cmd.TenantId,
		ReferenceModule: cmd.ReferenceModule,
		ReferenceId:     cmd.ReferenceId,
		Channel:         ChannelOnline,
		Method:          MethodCard,
		Amount:          amount,
		Currency:        currency,
		Status:          StatusPending,
		ReturnTo:        cmd.ReturnTo,
		CreatedAt:       now,
		UpdatedAt:       now,
		Revision:        1,
	}
	started, gwErr := m.gw.Start(GatewayStart{
		PaymentId: p.Id,
		Amount:    amount,
		Currency:  currency,
		Payer:     Payer{Name: cmd.PayerName, Email: cmd.PayerEmail, Phone: cmd.PayerPhone},
	})
	if gwErr != nil {
		p.Status = StatusFailed
		if err := m.create(&p); err != nil {
			return StartOnlinePaymentResult{}, err
		}
		return StartOnlinePaymentResult{}, ErrGatewayFailed
	}
	p.ProviderToken = started.Token
	p.Provider = m.gw.Provider()
	if err := m.create(&p); err != nil {
		return StartOnlinePaymentResult{}, err
	}
	return StartOnlinePaymentResult{PaymentId: p.Id, RedirectUrl: started.RedirectURL, Token: started.Token, TokenField: started.TokenField}, nil
}

// StartTerminalPayment crea el pago PENDING y envía el monto a la máquina de la caja del módulo de la
// referencia. Repetir el mismo id con la misma referencia devuelve el pago guardado.
func (m *Module) StartTerminalPayment(cmd StartTerminalPaymentArgs) (Payment, error) {
	if m.term == nil {
		return Payment{}, ErrTerminalUnavailable
	}
	if cmd.TenantId == "" || cmd.Id == "" || cmd.ReferenceModule == "" || cmd.ReferenceId == "" || cmd.Method == "" {
		return Payment{}, ErrMissingArgs
	}
	existing, err := m.repo.paymentAnyTenant(cmd.Id)
	if err == nil {
		if existing.TenantId == cmd.TenantId && existing.ReferenceModule == cmd.ReferenceModule && existing.ReferenceId == cmd.ReferenceId {
			return existing, nil
		}
		return Payment{}, ErrIdTaken
	}
	if model.KindOf(err) != model.ErrorNotFound {
		return Payment{}, err
	}
	if cmd.Method != MethodDebit && cmd.Method != MethodCredit {
		return Payment{}, ErrInvalidMethod
	}
	src, ok := m.source(cmd.ReferenceModule)
	if !ok || src.Device == "" {
		return Payment{}, ErrNoTerminalDevice
	}
	amount, currency, err := m.Due(cmd.TenantId, cmd.ReferenceModule, cmd.ReferenceId)
	if err != nil {
		return Payment{}, err
	}

	now := time.NowUnix()
	p := Payment{
		Id:              cmd.Id,
		TenantId:        cmd.TenantId,
		ReferenceModule: cmd.ReferenceModule,
		ReferenceId:     cmd.ReferenceId,
		Channel:         ChannelTerminal,
		Method:          cmd.Method,
		Amount:          amount,
		Currency:        currency,
		Status:          StatusPending,
		Provider:        m.term.Provider(),
		CreatedAt:       now,
		UpdatedAt:       now,
		Revision:        1,
	}
	pushed, err := m.term.Push(TerminalPush{RequestId: p.Id, Device: src.Device, Amount: amount, Currency: currency, Method: cmd.Method})
	if err != nil {
		p.Status = StatusFailed
		_ = m.create(&p)
		return Payment{}, ErrTerminalFailed
	}
	p.ProviderToken = pushed.Token
	if err := m.create(&p); err != nil {
		return Payment{}, err
	}
	return p, nil
}

// RefreshTerminalPayment consulta la máquina y asienta el resultado de un pago PENDING. Un pago que ya
// no está PENDING se devuelve tal cual.
func (m *Module) RefreshTerminalPayment(tenantId, id string) (Payment, error) {
	p, err := m.repo.paymentById(tenantId, id)
	if err != nil {
		return Payment{}, err
	}
	if p.Channel != ChannelTerminal {
		return Payment{}, ErrNotTerminal
	}
	if p.Status != StatusPending {
		return p, nil
	}
	if m.term == nil {
		return Payment{}, ErrTerminalUnavailable
	}
	res, err := m.term.Status(p.ProviderToken)
	if err != nil {
		return p, ErrTerminalFailed
	}
	return m.settleTerminal(p, res, false)
}

// settleTerminal asienta lo que informa la máquina. fromReconcile: un resultado aún pendiente vence.
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

// CompleteOnlinePayment asienta el resultado que el adaptador ya verificó. Es idempotente: un pago que ya no
// está PENDING se devuelve tal cual.
func (m *Module) CompleteOnlinePayment(r OnlineResult) (Payment, error) {
	if m.gw == nil {
		return Payment{}, ErrGatewayUnavailable
	}
	if r.PaymentId == "" {
		return Payment{}, ErrMissingArgs
	}
	p, err := m.repo.paymentAnyTenant(r.PaymentId)
	if err != nil {
		return Payment{}, err
	}
	if p.Status != StatusPending {
		return p, nil
	}
	return m.settle(p, r.Result, false)
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
func (m *Module) RefundPayment(cmd RefundPaymentArgs) (Refund, error) {
	if cmd.TenantId == "" || cmd.Id == "" || cmd.PaymentId == "" || cmd.Reason == "" {
		return Refund{}, ErrMissingArgs
	}
	existing, err := m.repo.refundAnyTenant(cmd.Id)
	if err == nil {
		if existing.TenantId == cmd.TenantId && existing.PaymentId == cmd.PaymentId {
			return existing, nil
		}
		return Refund{}, ErrIdTaken
	}
	if model.KindOf(err) != model.ErrorNotFound {
		return Refund{}, err
	}
	p, err := m.repo.paymentById(cmd.TenantId, cmd.PaymentId)
	if err != nil {
		return Refund{}, err
	}
	if p.Status != StatusPaid && p.Status != StatusPartiallyRefunded {
		return Refund{}, ErrInvalidTransition
	}
	if cmd.Amount <= 0 || cmd.Amount > p.Amount-p.RefundedAmount {
		return Refund{}, ErrRefundExceeds
	}

	ref := Refund{
		Id:        cmd.Id,
		TenantId:  cmd.TenantId,
		PaymentId: cmd.PaymentId,
		Amount:    cmd.Amount,
		Reason:    cmd.Reason,
		CreatedBy: cmd.CreatedBy,
		CreatedAt: time.NowUnix(),
	}
	if p.Channel == ChannelOnline {
		if m.gw == nil {
			return Refund{}, ErrGatewayUnavailable
		}
		res, gwErr := m.gw.Refund(p.ProviderToken, cmd.Amount)
		if gwErr != nil {
			return Refund{}, ErrGatewayFailed
		}
		ref.Kind = res.Kind
		ref.AuthorizationCode = res.AuthorizationCode
		ref.Amount = res.Amount
	} else {
		if !isCounterMethod(cmd.Method) {
			return Refund{}, ErrInvalidMethod
		}
		ref.Kind = RefundKindManual
		ref.Method = cmd.Method
	}
	if err := ref.Validate(byte(model.ActionCreate)); err != nil {
		return Refund{}, err
	}

	event := EventRefundPartial
	if p.RefundedAmount+ref.Amount >= p.Amount {
		event = EventRefundTotal
	}
	// El estado ya se verificó (PAID o PARTIALLY_REFUNDED): la transición existe.
	next, _ := Transition(p.Status, event)
	expected := p.Revision
	p.RefundedAmount += ref.Amount
	p.Status = next
	p.Revision++
	p.UpdatedAt = ref.CreatedAt
	err = m.db.Tx(func(tx *orm.DB) error {
		if err := tx.Create(&ref); err != nil {
			return err
		}
		return m.repo.updatePaymentTx(tx, &p, expected)
	})
	if err != nil {
		return Refund{}, err
	}
	if m.pub != nil {
		fact, err := refundFact(p, ref)
		if err != nil {
			return Refund{}, err
		}
		if err := money.Publish(m.pub, fact); err != nil {
			return Refund{}, err
		}
	}
	m.publish(EventPaymentRefunded, &ref)
	return ref, nil
}

// ReconcilePayments cierra los pagos online y de máquina que siguen PENDING después
// de PendingTimeout (diagrama §8). Devuelve cuántos cerró.
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
