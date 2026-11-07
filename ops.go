package paymentmanager

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/time"
	"webtyp.com/view"
	"webtyp.com/rpc"
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
var (
	ResourcePayment       = model.ResourceOf(&PaymentModel)
	ResourcePaymentOnline = model.FeatureResource("payment_online")
	ResourceRefund        = model.ResourceOf(&RefundModel)
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
	reg.Operation(OpListPayments, m.opListPayments).Requires(ResourcePayment, model.Read).Accepts(&view.ListArgs{})
	reg.Operation(OpRefundPayment, m.opRefundPayment).Requires(ResourceRefund, model.Create).Accepts(&RefundPaymentArgs{})
	reg.Operation(OpListRefunds, m.opListRefunds).Requires(ResourceRefund, model.Read).Accepts(&ListRefundsArgs{})
	reg.Operation(OpStartTerminalPayment, m.opStartTerminalPayment).Requires(ResourcePayment, model.Create).Accepts(&StartTerminalPaymentArgs{})
	reg.Operation(OpRefreshTerminalPayment, m.opRefreshTerminalPayment).Requires(ResourcePayment, model.Update).Accepts(&GetPaymentArgs{})
	reg.Operation(OpReconcilePayments, m.opReconcilePayments).Requires(ResourcePayment, model.Update).Accepts(&ReconcilePaymentsArgs{})
}

// StatusFor es el código HTTP de un error del módulo (tabla del plan, §5).
// decode lee y valida los argumentos; false = ya respondió 400.
func decode(ctx router.Context, args interface {
	model.Decodable
	Validate(byte) error
}, action byte) bool {
	if err := ctx.Decode(args); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return false
	}
	if err := args.Validate(action); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
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
	if err := ctx.Decode(&a); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	a.TenantId = m.tenantID
	if err := a.Validate(model.ActionRead); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	amount, currency, err := m.Due(a.TenantId, a.ReferenceModule, a.ReferenceId)
	switch err {
	case nil:
		_ = ctx.Encode(&ChargeResult{Amount: amount, Currency: currency, Payable: true, Terminal: m.hasTerminal(a.ReferenceModule)})
	case ErrAlreadyPaid, ErrNotPayable:
		_ = ctx.Encode(&ChargeResult{Payable: false})
	default:
		rpc.WriteError(ctx, err)
	}
}

func (m *Module) opCreateCounterPayment(ctx router.Context) {
	var a CreateCounterPaymentArgs
	if err := ctx.Decode(&a); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	a.TenantId = m.tenantID
	if err := a.Validate(model.ActionCreate); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	a.CreatedBy = ctx.UserID()
	p, err := m.CreateCounterPayment(a)
	if err != nil {
		rpc.WriteError(ctx, err)
		return
	}
	_ = ctx.Encode(public(p))
}

func (m *Module) opStartOnlinePayment(ctx router.Context) {
	var a StartOnlinePaymentArgs
	if err := ctx.Decode(&a); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	a.TenantId = m.tenantID
	if err := a.Validate(model.ActionCreate); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	a.CreatedBy = ctx.UserID()
	res, err := m.StartOnlinePayment(a)
	if err != nil {
		rpc.WriteError(ctx, err)
		return
	}
	_ = ctx.Encode(&res)
}

func (m *Module) opGetPaymentStatus(ctx router.Context) {
	var a GetPaymentArgs
	if err := ctx.Decode(&a); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	a.TenantId = m.tenantID
	if err := a.Validate(model.ActionRead); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	p, err := m.GetPayment(a.TenantId, a.Id)
	if err != nil {
		rpc.WriteError(ctx, err)
		return
	}
	_ = ctx.Encode(&PaymentStatusResult{Id: p.Id, Status: p.Status})
}

func (m *Module) opGetPayment(ctx router.Context) {
	var a GetPaymentArgs
	if err := ctx.Decode(&a); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	a.TenantId = m.tenantID
	if err := a.Validate(model.ActionRead); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	p, err := m.GetPayment(a.TenantId, a.Id)
	if err != nil {
		rpc.WriteError(ctx, err)
		return
	}
	_ = ctx.Encode(public(p))
}

func (m *Module) opListPaymentsByReference(ctx router.Context) {
	var a ListPaymentsByReferenceArgs
	if err := ctx.Decode(&a); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	a.TenantId = m.tenantID
	if err := a.Validate(model.ActionRead); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	list, err := m.ListPaymentsByReference(a.TenantId, a.ReferenceModule, a.ReferenceId)
	if err != nil {
		rpc.WriteError(ctx, err)
		return
	}
	_ = ctx.Encode(publicList(list))
}

func (m *Module) opListPayments(ctx router.Context) {
	var a view.ListArgs
	if err := ctx.Decode(&a); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	list, err := m.listPaymentsPaged(m.tenantID, a.Search, a.After, view.Cap(a.Limit))
	if err != nil {
		rpc.WriteError(ctx, err)
		return
	}
	res := make(PaymentList, 0, len(list))
	for i := range list {
		res = append(res, public(list[i]))
	}
	_ = ctx.Encode(&res)
}

func (m *Module) opRefundPayment(ctx router.Context) {
	var a RefundPaymentArgs
	if err := ctx.Decode(&a); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	a.TenantId = m.tenantID
	if err := a.Validate(model.ActionCreate); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	a.CreatedBy = ctx.UserID()
	ref, err := m.RefundPayment(a)
	if err != nil {
		rpc.WriteError(ctx, err)
		return
	}
	_ = ctx.Encode(&ref)
}

func (m *Module) opListRefunds(ctx router.Context) {
	var a ListRefundsArgs
	if err := ctx.Decode(&a); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	a.TenantId = m.tenantID
	if err := a.Validate(model.ActionRead); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	refunds, err := m.ListRefunds(a.TenantId, a.PaymentId)
	if err != nil {
		rpc.WriteError(ctx, err)
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
	if err := ctx.Decode(&a); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	a.TenantId = m.tenantID
	if err := a.Validate(model.ActionCreate); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	a.CreatedBy = ctx.UserID()
	p, err := m.StartTerminalPayment(a)
	if err != nil {
		rpc.WriteError(ctx, err)
		return
	}
	_ = ctx.Encode(public(p))
}

func (m *Module) opRefreshTerminalPayment(ctx router.Context) {
	var a GetPaymentArgs
	if err := ctx.Decode(&a); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	a.TenantId = m.tenantID
	if err := a.Validate(model.ActionUpdate); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	p, err := m.RefreshTerminalPayment(a.TenantId, a.Id)
	if err != nil {
		rpc.WriteError(ctx, err)
		return
	}
	_ = ctx.Encode(public(p))
}

func (m *Module) opReconcilePayments(ctx router.Context) {
	var a ReconcilePaymentsArgs
	if err := ctx.Decode(&a); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	a.TenantId = m.tenantID
	if err := a.Validate(model.ActionUpdate); err != nil {
		rpc.WriteError(ctx, ErrCannotRead)
		return
	}
	closed, err := m.ReconcilePayments(a.TenantId, time.NowUnix())
	if err != nil {
		rpc.WriteError(ctx, err)
		return
	}
	_ = ctx.Encode(&ReconcileResult{Closed: int64(closed)})
}
