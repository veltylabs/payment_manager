package paymentmanager

import (
	"webtyp.com/time"
	"webtyp.com/model"
	"webtyp.com/router"
)

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
	OpReconcileOnlinePayments = "reconcile_online_payments"
)

const (
	ResourcePayment       = "payment"
	ResourcePaymentOnline = "payment_online"
	ResourceRefund        = "refund"
)

func (m *Module) ModelName() string {
	return ModelName
}

func (m *Module) MountOperations(reg router.OperationRegistry) {
	reg.Operation(OpGetCharge, m.getCharge).
		Requires(model.Resource(ResourcePaymentOnline), model.Action(model.ActionRead)).
		Accepts(&GetChargeArgs{})

	reg.Operation(OpCreateCounterPayment, m.createCounterPayment).
		Requires(model.Resource(ResourcePayment), model.Action(model.ActionCreate)).
		Accepts(&CreateCounterPaymentArgs{})

	reg.Operation(OpStartOnlinePayment, m.startOnlinePayment).
		Requires(model.Resource(ResourcePaymentOnline), model.Action(model.ActionCreate)).
		Accepts(&StartOnlinePaymentArgs{})

	reg.Operation(OpGetPaymentStatus, m.getPaymentStatus).
		Requires(model.Resource(ResourcePaymentOnline), model.Action(model.ActionRead)).
		Accepts(&GetPaymentArgs{})

	reg.Operation(OpGetPayment, m.getPayment).
		Requires(model.Resource(ResourcePayment), model.Action(model.ActionRead)).
		Accepts(&GetPaymentArgs{})

	reg.Operation(OpListPaymentsByReference, m.listPaymentsByReference).
		Requires(model.Resource(ResourcePayment), model.Action(model.ActionRead)).
		Accepts(&ListPaymentsByReferenceArgs{})

	reg.Operation(OpListPayments, m.listPayments).
		Requires(model.Resource(ResourcePayment), model.Action(model.ActionRead)).
		Accepts(&ListPaymentsArgs{})

	reg.Operation(OpRefundPayment, m.refundPayment).
		Requires(model.Resource(ResourceRefund), model.Action(model.ActionCreate)).
		Accepts(&RefundPaymentArgs{})

	reg.Operation(OpListRefunds, m.listRefunds).
		Requires(model.Resource(ResourceRefund), model.Action(model.ActionRead)).
		Accepts(&ListRefundsArgs{})

	reg.Operation(OpReconcileOnlinePayments, m.reconcileOnlinePayments).
		Requires(model.Resource(ResourcePayment), model.Action(model.ActionUpdate)).
		Accepts(&ReconcileOnlinePaymentsArgs{})
}

func ErrorStatus(err error) int {
	if err == ErrNotFound || err == ErrRefundNotFound {
		return 404
	}
	switch err {
	case ErrMissingArgs, ErrUnknownReference, ErrNotPayable, ErrUnsupportedCurrency, ErrInvalidMethod, ErrInvalidTransition, ErrRefundExceeds:
		return 400
	case ErrIdTaken, ErrAlreadyPaid, ErrNotPaid, ErrConflict:
		return 409
	case ErrGatewayUnavailable, ErrGatewayFailed, ErrAmountMismatch:
		return 500
	}
	return 500
}

func writeError(ctx router.Context, err error) {
	ctx.WriteStatus(ErrorStatus(err))
	ctx.Write([]byte(err.Error()))
}

func (m *Module) getCharge(ctx router.Context) {
	var args GetChargeArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	if err := args.Validate(byte(model.Action(model.ActionRead))); err != nil {
		ctx.WriteStatus(400)
		ctx.Write([]byte(err.Error()))
		return
	}

	amt, cur, pay, err := m.Charge(args.TenantId, args.ReferenceModule, args.ReferenceId)
	if err != nil {
		writeError(ctx, err)
		return
	}

	res := ChargeResult{Amount: amt, Currency: cur, Payable: pay}
	ctx.Encode(&res)
}

func (m *Module) createCounterPayment(ctx router.Context) {
	var args CreateCounterPaymentArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	if err := args.Validate(byte(model.Action(model.ActionCreate))); err != nil {
		ctx.WriteStatus(400)
		ctx.Write([]byte(err.Error()))
		return
	}

	p, err := m.CreateCounterPayment(args)
	if err != nil {
		writeError(ctx, err)
		return
	}
	p.ProviderToken = ""
	ctx.Encode(&p)
}

func (m *Module) startOnlinePayment(ctx router.Context) {
	var args StartOnlinePaymentArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	if err := args.Validate(byte(model.Action(model.ActionCreate))); err != nil {
		ctx.WriteStatus(400)
		ctx.Write([]byte(err.Error()))
		return
	}

	res, err := m.StartOnlinePayment(args)
	if err != nil {
		writeError(ctx, err)
		return
	}
	ctx.Encode(&res)
}

func (m *Module) getPaymentStatus(ctx router.Context) {
	var args GetPaymentArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	if err := args.Validate(byte(model.Action(model.ActionRead))); err != nil {
		ctx.WriteStatus(400)
		ctx.Write([]byte(err.Error()))
		return
	}

	p, err := m.GetPayment(args)
	if err != nil {
		writeError(ctx, err)
		return
	}
	res := PaymentStatusResult{Id: p.Id, Status: p.Status}
	ctx.Encode(&res)
}

func (m *Module) getPayment(ctx router.Context) {
	var args GetPaymentArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	if err := args.Validate(byte(model.Action(model.ActionRead))); err != nil {
		ctx.WriteStatus(400)
		ctx.Write([]byte(err.Error()))
		return
	}

	p, err := m.GetPayment(args)
	if err != nil {
		writeError(ctx, err)
		return
	}
	p.ProviderToken = ""
	ctx.Encode(&p)
}


func (m *Module) listPaymentsByReference(ctx router.Context) {
	var args ListPaymentsByReferenceArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	if err := args.Validate(byte(model.Action(model.ActionRead))); err != nil {
		ctx.WriteStatus(400)
		ctx.Write([]byte(err.Error()))
		return
	}

	list, err := m.ListPaymentsByReference(args)
	if err != nil {
		writeError(ctx, err)
		return
	}
	for i := range list {
		list[i].ProviderToken = ""
	}

	listObj := make(PaymentList, len(list))
	for i, p := range list {
		listObj[i] = p
	}
	ctx.Encode(&listObj)
}

func (m *Module) listPayments(ctx router.Context) {
	var args ListPaymentsArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	if err := args.Validate(byte(model.Action(model.ActionRead))); err != nil {
		ctx.WriteStatus(400)
		ctx.Write([]byte(err.Error()))
		return
	}

	list, err := m.ListPayments(args)
	if err != nil {
		writeError(ctx, err)
		return
	}
	for i := range list {
		list[i].ProviderToken = ""
	}

	listObj := make(PaymentList, len(list))
	for i, p := range list {
		listObj[i] = p
	}
	ctx.Encode(&listObj)
}

func (m *Module) refundPayment(ctx router.Context) {
	var args RefundPaymentArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	if err := args.Validate(byte(model.Action(model.ActionCreate))); err != nil {
		ctx.WriteStatus(400)
		ctx.Write([]byte(err.Error()))
		return
	}

	r, err := m.RefundPayment(args)
	if err != nil {
		writeError(ctx, err)
		return
	}
	ctx.Encode(&r)
}


func (m *Module) listRefunds(ctx router.Context) {
	var args ListRefundsArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	if err := args.Validate(byte(model.Action(model.ActionRead))); err != nil {
		ctx.WriteStatus(400)
		ctx.Write([]byte(err.Error()))
		return
	}

	list, err := m.ListRefunds(args)
	if err != nil {
		writeError(ctx, err)
		return
	}

	listObj := make(RefundList, len(list))
	for i, r := range list {
		listObj[i] = r
	}
	ctx.Encode(&listObj)
}

func (m *Module) reconcileOnlinePayments(ctx router.Context) {
	var args ReconcileOnlinePaymentsArgs
	if err := ctx.Decode(&args); err != nil {
		ctx.WriteStatus(400)
		return
	}
	if err := args.Validate(byte(model.Action(model.ActionUpdate))); err != nil {
		ctx.WriteStatus(400)
		ctx.Write([]byte(err.Error()))
		return
	}

	now := time.Now() / 1_000_000_000
	res, err := m.ReconcileOnlinePayments(args.TenantId, int64(now))
	if err != nil {
		writeError(ctx, err)
		return
	}
	ctx.Encode(&res)
}

var _ router.OperationModule = (*Module)(nil)
