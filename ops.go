package paymentmanager

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/time"
)

// Operaciones del módulo; en el cable van como "payment_manager.<op>".
const (
	OpGetCharge               = "get_charge"
	OpCreateCounterPayment    = "create_counter_payment"
	OpStartOnlinePayment      = "start_online_payment"
	OpGetPaymentStatus        = "get_payment_status"
	OpGetPayment              = "get_payment"
	OpListPaymentsByReference = "list_payments_by_reference"
	OpListPayments            = "list_payments"
	OpRefundPayment           = "refund_payment"
	OpListRefunds             = "list_refunds"
	OpStartTerminalPayment    = "start_terminal_payment"
	OpRefreshTerminalPayment  = "refresh_terminal_payment"
	OpReconcilePayments       = "reconcile_payments"
)

// Recursos RBAC. La app concede ResourcePaymentOnline a la identidad anónima
// para la reserva online; el resto, solo al personal.
const (
	ResourcePayment       = "payment"
	ResourcePaymentOnline = "payment_online"
	ResourceRefund        = "refund"
)

// QueryPayment y QueryStatus son los parámetros de la query con que la ruta de
// retorno de la pasarela devuelve el navegador a la app:
// /?payment=<id>&status=<estado>#<pantalla>.
const (
	QueryPayment = "payment"
	QueryStatus  = "status"
)

func (m *Module) ModelName() string { return ModelName }

var _ router.OperationModule = (*Module)(nil)

func (m *Module) MountOperations(reg router.OperationRegistry) {
	reg.Operation(OpGetCharge, m.opGetCharge).Requires(ResourcePaymentOnline, model.Read).Accepts(&GetChargeArgs{})
	reg.Operation(OpCreateCounterPayment, m.opCreateCounterPayment).Requires(ResourcePayment, model.Create).Accepts(&CreateCounterPaymentArgs{})
	reg.Operation(OpStartOnlinePayment, m.opStartOnlinePayment).Requires(ResourcePaymentOnline, model.Create).Accepts(&StartOnlinePaymentArgs{})
	reg.Operation(OpGetPaymentStatus, m.opGetPaymentStatus).Requires(ResourcePaymentOnline, model.Read).Accepts(&GetPaymentArgs{})
	reg.Operation(OpGetPayment, m.opGetPayment).Requires(ResourcePayment, model.Read).Accepts(&GetPaymentArgs{})
	reg.Operation(OpListPaymentsByReference, m.opListPaymentsByReference).Requires(ResourcePayment, model.Read).Accepts(&ListPaymentsByReferenceArgs{})
	reg.Operation(OpListPayments, m.opListPayments).Requires(ResourcePayment, model.Read).Accepts(&ListPaymentsArgs{})
	reg.Operation(OpRefundPayment, m.opRefundPayment).Requires(ResourceRefund, model.Create).Accepts(&RefundPaymentArgs{})
	reg.Operation(OpListRefunds, m.opListRefunds).Requires(ResourceRefund, model.Read).Accepts(&ListRefundsArgs{})
	reg.Operation(OpStartTerminalPayment, m.opStartTerminalPayment).Requires(ResourcePayment, model.Create).Accepts(&StartTerminalPaymentArgs{})
	reg.Operation(OpRefreshTerminalPayment, m.opRefreshTerminalPayment).Requires(ResourcePayment, model.Update).Accepts(&GetPaymentArgs{})
	reg.Operation(OpReconcilePayments, m.opReconcilePayments).Requires(ResourcePayment, model.Update).Accepts(&ReconcilePaymentsArgs{})
}

// StatusFor es el código HTTP de un error del módulo (tabla del plan, §5).
func StatusFor(err error) int {
	switch err {
	case ErrNotFound, ErrRefundNotFound:
		return 404
	case ErrMissingArgs, ErrUnknownReference, ErrNotPayable, ErrUnsupportedCurrency,
		ErrInvalidMethod, ErrInvalidTransition, ErrRefundExceeds,
		ErrNoTerminalDevice, ErrTerminalRequired, ErrNotTerminal:
		return 400
	case ErrIdTaken, ErrAlreadyPaid, ErrNotPaid, ErrConflict:
		return 409
	}
	return 500
}

func writeError(ctx router.Context, err error) {
	ctx.WriteStatus(StatusFor(err))
	_, _ = ctx.Write([]byte(err.Error()))
}

// decode lee y valida los argumentos; false = ya respondió 400.
func decode(ctx router.Context, args interface {
	model.Decodable
	Validate(byte) error
}, action byte) bool {
	if err := ctx.Decode(args); err != nil {
		ctx.WriteStatus(400)
		_, _ = ctx.Write([]byte(err.Error()))
		return false
	}
	if err := args.Validate(action); err != nil {
		ctx.WriteStatus(400)
		_, _ = ctx.Write([]byte(err.Error()))
		return false
	}
	return true
}

// public devuelve el pago sin el token de la pasarela, que nunca sale del servidor.
func public(p Payment) *Payment {
	p.ProviderToken = ""
	return &p
}

func publicList(payments []Payment) *PaymentList {
	list := make(PaymentList, len(payments))
	for i := range payments {
		list[i] = public(payments[i])
	}
	return &list
}

func (m *Module) opGetCharge(ctx router.Context) {
	var a GetChargeArgs
	if !decode(ctx, &a, model.ActionRead) {
		return
	}
	amount, currency, err := m.Due(a.TenantId, a.ReferenceModule, a.ReferenceId)
	switch err {
	case nil:
		_ = ctx.Encode(&ChargeResult{Amount: amount, Currency: currency, Payable: true, Terminal: m.hasTerminal(a.ReferenceModule)})
	case ErrAlreadyPaid, ErrNotPayable:
		_ = ctx.Encode(&ChargeResult{Payable: false})
	default:
		writeError(ctx, err)
	}
}

func (m *Module) opCreateCounterPayment(ctx router.Context) {
	var a CreateCounterPaymentArgs
	if !decode(ctx, &a, model.ActionCreate) {
		return
	}
	a.CreatedBy = ctx.UserID()
	p, err := m.CreateCounterPayment(a)
	if err != nil {
		writeError(ctx, err)
		return
	}
	_ = ctx.Encode(public(p))
}

func (m *Module) opStartOnlinePayment(ctx router.Context) {
	var a StartOnlinePaymentArgs
	if !decode(ctx, &a, model.ActionCreate) {
		return
	}
	a.CreatedBy = ctx.UserID()
	res, err := m.StartOnlinePayment(a)
	if err != nil {
		writeError(ctx, err)
		return
	}
	_ = ctx.Encode(&res)
}

func (m *Module) opGetPaymentStatus(ctx router.Context) {
	var a GetPaymentArgs
	if !decode(ctx, &a, model.ActionRead) {
		return
	}
	p, err := m.GetPayment(a.TenantId, a.Id)
	if err != nil {
		writeError(ctx, err)
		return
	}
	_ = ctx.Encode(&PaymentStatusResult{Id: p.Id, Status: p.Status})
}

func (m *Module) opGetPayment(ctx router.Context) {
	var a GetPaymentArgs
	if !decode(ctx, &a, model.ActionRead) {
		return
	}
	p, err := m.GetPayment(a.TenantId, a.Id)
	if err != nil {
		writeError(ctx, err)
		return
	}
	_ = ctx.Encode(public(p))
}

func (m *Module) opListPaymentsByReference(ctx router.Context) {
	var a ListPaymentsByReferenceArgs
	if !decode(ctx, &a, model.ActionRead) {
		return
	}
	list, err := m.ListPaymentsByReference(a.TenantId, a.ReferenceModule, a.ReferenceId)
	if err != nil {
		writeError(ctx, err)
		return
	}
	_ = ctx.Encode(publicList(list))
}

func (m *Module) opListPayments(ctx router.Context) {
	var a ListPaymentsArgs
	if !decode(ctx, &a, model.ActionRead) {
		return
	}
	list, err := m.ListPayments(a.TenantId, time.Unix(a.From), time.Unix(a.To))
	if err != nil {
		writeError(ctx, err)
		return
	}
	_ = ctx.Encode(publicList(list))
}

func (m *Module) opRefundPayment(ctx router.Context) {
	var a RefundPaymentArgs
	if !decode(ctx, &a, model.ActionCreate) {
		return
	}
	a.CreatedBy = ctx.UserID()
	ref, err := m.RefundPayment(a)
	if err != nil {
		writeError(ctx, err)
		return
	}
	_ = ctx.Encode(&ref)
}

func (m *Module) opListRefunds(ctx router.Context) {
	var a ListRefundsArgs
	if !decode(ctx, &a, model.ActionRead) {
		return
	}
	refunds, err := m.ListRefunds(a.TenantId, a.PaymentId)
	if err != nil {
		writeError(ctx, err)
		return
	}
	list := make(RefundList, len(refunds))
	for i := range refunds {
		list[i] = &refunds[i]
	}
	_ = ctx.Encode(&list)
}

func (m *Module) opStartTerminalPayment(ctx router.Context) {
	var a StartTerminalPaymentArgs
	if !decode(ctx, &a, model.ActionCreate) {
		return
	}
	a.CreatedBy = ctx.UserID()
	p, err := m.StartTerminalPayment(a)
	if err != nil {
		writeError(ctx, err)
		return
	}
	_ = ctx.Encode(public(p))
}

func (m *Module) opRefreshTerminalPayment(ctx router.Context) {
	var a GetPaymentArgs
	if !decode(ctx, &a, model.ActionUpdate) {
		return
	}
	p, err := m.RefreshTerminalPayment(a.TenantId, a.Id)
	if err != nil {
		writeError(ctx, err)
		return
	}
	_ = ctx.Encode(public(p))
}

func (m *Module) opReconcilePayments(ctx router.Context) {
	var a ReconcilePaymentsArgs
	if !decode(ctx, &a, model.ActionUpdate) {
		return
	}
	closed, err := m.ReconcilePayments(a.TenantId, time.NowUnix())
	if err != nil {
		writeError(ctx, err)
		return
	}
	_ = ctx.Encode(&ReconcileResult{Closed: int64(closed)})
}
