package paymentmanager

import (
	"webtyp.com/events"
	"webtyp.com/fmt"
	"webtyp.com/model"
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
	ReturnURL string           // requerido con Gateway: URL absoluta de la ruta de retorno
	Charges   []ChargeSource   // requerido: quién responde el monto de cada módulo de referencia
}

// Module registra, confirma y devuelve pagos.
type Module struct {
	db      *orm.DB
	repo    *repository
	pub     events.Publisher
	gw      Gateway
	retURL  string
	charges []ChargeSource
}

// New conecta el módulo a un *orm.DB ya migrado (ver el subpaquete migrate).
func New(db *orm.DB, deps Deps) (*Module, error) {
	if len(deps.Charges) == 0 {
		return nil, fmt.Err("payment_manager: Deps.Charges is required")
	}
	if deps.Gateway != nil && deps.ReturnURL == "" {
		return nil, fmt.Err("payment_manager: Deps.ReturnURL is required with a Gateway")
	}
	return &Module{
		db:      db,
		repo:    &repository{db: db},
		pub:     deps.Publisher,
		gw:      deps.Gateway,
		retURL:  deps.ReturnURL,
		charges: deps.Charges,
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

// Charge pregunta al módulo dueño de la referencia cuánto se cobra. El monto
// nunca viene del cliente.
func (m *Module) Charge(tenantId, referenceModule, referenceId string) (amount int64, currency string, payable bool, err error) {
	var reader ChargeReader
	for _, c := range m.charges {
		if c.Module == referenceModule {
			reader = c.Reader
			break
		}
	}
	if reader == nil {
		return 0, "", false, ErrUnknownReference
	}
	amount, currency, payable, err = reader.Charge(tenantId, referenceId)
	if err != nil {
		return 0, "", false, err
	}
	if currency != CurrencyCLP && currency != CurrencyUSD {
		return 0, "", false, ErrUnsupportedCurrency
	}
	return amount, currency, payable, nil
}

// referencePaid informa si la referencia ya tiene un pago vigente.
func (m *Module) referencePaid(tenantId, referenceModule, referenceId string) (bool, error) {
	payments, err := m.repo.paymentsByReference(tenantId, referenceModule, referenceId)
	if err != nil {
		return false, err
	}
	for _, p := range payments {
		if p.Status == StatusPaid || p.Status == StatusPartiallyRefunded {
			return true, nil
		}
	}
	return false, nil
}

// chargeFor resuelve el monto de una referencia que aún no está pagada.
func (m *Module) chargeFor(tenantId, referenceModule, referenceId string) (int64, string, error) {
	paid, err := m.referencePaid(tenantId, referenceModule, referenceId)
	if err != nil {
		return 0, "", err
	}
	if paid {
		return 0, "", ErrAlreadyPaid
	}
	amount, currency, payable, err := m.Charge(tenantId, referenceModule, referenceId)
	if err != nil {
		return 0, "", err
	}
	if !payable {
		return 0, "", ErrNotPayable
	}
	return amount, currency, nil
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
	if err != ErrNotFound {
		return Payment{}, err
	}
	if !isCounterMethod(cmd.Method) {
		return Payment{}, ErrInvalidMethod
	}
	amount, currency, err := m.chargeFor(cmd.TenantId, cmd.ReferenceModule, cmd.ReferenceId)
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
		ReceiptNumber:   cmd.ReceiptNumber,
		CreatedAt:       now,
		PaidAt:          now,
		UpdatedAt:       now,
		Revision:        1,
	}
	if err := m.create(&p); err != nil {
		return Payment{}, err
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
	} else if err != ErrNotFound {
		return StartOnlinePaymentResult{}, err
	}
	amount, currency, err := m.chargeFor(cmd.TenantId, cmd.ReferenceModule, cmd.ReferenceId)
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
		Method:          MethodWebpay,
		Amount:          amount,
		Currency:        currency,
		Status:          StatusPending,
		ReturnTo:        cmd.ReturnTo,
		CreatedAt:       now,
		UpdatedAt:       now,
		Revision:        1,
	}
	started, gwErr := m.gw.Start(GatewayStart{BuyOrder: p.Id, SessionId: p.Id, Amount: amount, Currency: currency, ReturnURL: m.retURL})
	if gwErr != nil {
		p.Status = StatusFailed
		if err := m.create(&p); err != nil {
			return StartOnlinePaymentResult{}, err
		}
		return StartOnlinePaymentResult{}, ErrGatewayFailed
	}
	p.ProviderToken = started.Token
	if err := m.create(&p); err != nil {
		return StartOnlinePaymentResult{}, err
	}
	return StartOnlinePaymentResult{PaymentId: p.Id, RedirectUrl: started.RedirectURL, Token: started.Token}, nil
}

// CompleteOnlinePayment termina un pago online con lo que devolvió la
// pasarela (ver OnlineReturn). Es idempotente: un pago que ya no está PENDING
// se devuelve tal cual.
func (m *Module) CompleteOnlinePayment(ret OnlineReturn) (Payment, error) {
	if m.gw == nil {
		return Payment{}, ErrGatewayUnavailable
	}
	var p Payment
	var err error
	switch {
	case ret.AbortToken != "":
		p, err = m.repo.paymentByProviderToken(ret.AbortToken)
	case ret.Token != "":
		p, err = m.repo.paymentByProviderToken(ret.Token)
	case ret.BuyOrder != "":
		p, err = m.repo.paymentAnyTenant(ret.BuyOrder)
	default:
		return Payment{}, ErrMissingArgs
	}
	if err != nil {
		return Payment{}, err
	}
	if p.Status != StatusPending {
		return p, nil
	}

	switch {
	case ret.AbortToken != "":
		return m.close(p, EventCancel)
	case ret.Token == "":
		return m.close(p, EventExpire)
	}
	result, gwErr := m.gw.Commit(ret.Token)
	if gwErr != nil {
		return p, ErrGatewayFailed
	}
	return m.settle(p, result, false)
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
	if err != ErrRefundNotFound {
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
	m.publish(EventPaymentRefunded, &ref)
	return ref, nil
}

// ReconcileOnlinePayments cierra los pagos online que siguen PENDING después
// de OnlinePendingTimeout (diagrama §8). Devuelve cuántos cerró. Sin pasarela
// no hay pagos online que cerrar.
func (m *Module) ReconcileOnlinePayments(tenantId string, now time.Unix) (int, error) {
	if m.gw == nil {
		return 0, nil
	}
	pending, err := m.repo.oldPendingOnlinePayments(tenantId, int64(now-OnlinePendingTimeout))
	if err != nil {
		return 0, err
	}
	closed := 0
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
