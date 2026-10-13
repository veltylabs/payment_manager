package ui

import (
	"webtyp.com/components/segmentedcontrol"
	"webtyp.com/components/targetlist"
	"webtyp.com/fmt"
	"webtyp.com/lang"
	"webtyp.com/layout/platformd"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/svg"
	"webtyp.com/view"
	"webtyp.com/widget"

	. "webtyp.com/dom"
	. "webtyp.com/html"

	paymentmanager "github.com/veltylabs/payment_manager"
)

// NamePayments es la identidad de widget de la pantalla Pagos.
const NamePayments = widget.Name("payments")

const (
	PartToolbar = widget.Part("toolbar")
	PartMessage = widget.Part("message")
	PartDetail  = widget.Part("detail")
	PartRefund  = widget.Part("refund")
)

var (
	clsPayRoot    = NamePayments.Root()
	clsPayToolbar = NamePayments.Class(PartToolbar)
	clsPayMessage = NamePayments.Class(PartMessage)
	clsPayDetail  = NamePayments.Class(PartDetail)
	clsPayRefund  = NamePayments.Class(PartRefund)
)

const (
	textReconcile     lang.Text = "Reconcile online payments"
	textClosed        lang.Text = "Closed payments:"
	textRefund        lang.Text = "Refund"
	textAmount        lang.Text = "Amount"
	textReason        lang.Text = "Reason"
	textRefunds       lang.Text = "Refunds"
	textInvalidAmount lang.Text = "Invalid amount"
)

// Browser construye la pantalla Pagos: los pagos de los últimos 30 días, el
// detalle del elegido con sus devoluciones, devolver y conciliar.
func Browser(caller router.Caller, ids model.IDGenerator, tenantID string) (platformd.UIModule, error) {
	return platformd.NewUIModule(ID, lang.Translate(NavLabel).String(), svg.Icon(ID), NewPaymentsView(caller, ids, tenantID)), nil
}

// PaymentsView es la pantalla Pagos.
type PaymentsView struct {
	Element // value embed — NEVER pointer

	caller    router.Caller
	ids       model.IDGenerator
	tenantID  string
	presenter view.Presenter
	list      *targetlist.TargetList
	message   *SignalString
	detail    *SignalNodes

	selected     *paymentmanager.Payment
	refundAmount *SignalString
	refundReason *SignalString
	refundMethod string
}

// NewPaymentsView arma la pantalla; Init carga los pagos.
func NewPaymentsView(caller router.Caller, ids model.IDGenerator, tenantID string) *PaymentsView {
	v := &PaymentsView{
		caller:       caller,
		ids:          ids,
		tenantID:     tenantID,
		presenter:    paymentmanager.NewView(caller, tenantID),
		message:      NewString(""),
		detail:       NewNodes(),
		refundAmount: NewString(""),
		refundReason: NewString(""),
	}
	v.list = &targetlist.TargetList{OnSelect: func(it view.Item) { v.show(it.ID) }}
	return v
}

func (v *PaymentsView) WidgetName() widget.Name { return NamePayments }
func (v *PaymentsView) WidgetKind() widget.Kind { return widget.Region }

func (v *PaymentsView) Init(_ Ctx) { v.reload() }

func (v *PaymentsView) Render() *Element {
	return Div().Set(clsPayRoot.AsAttr()).Child(
		Div().Set(clsPayToolbar.AsAttr()).Child(
			Button().Attr("type", "button").Attr("name", "reconcile").
				Text(lang.Translate(textReconcile).String()).
				OnClick(func(Event) { v.reconcile() }),
		),
		P().Set(clsPayMessage.AsAttr()).Attr("role", "status").BindText(v.message),
		v.list,
		Div().Set(clsPayDetail.AsAttr()).BindChildren(v.detail),
	)
}

func (v *PaymentsView) reload() {
	v.presenter.Reload(func(err error) {
		if err != nil {
			v.message.Set(err.Error())
			return
		}
		v.list.SetItems(v.presenter.Items())
	})
}

func (v *PaymentsView) reconcile() {
	out := &paymentmanager.ReconcileResult{}
	v.caller.Call(paymentmanager.ModelName+"."+paymentmanager.OpReconcileOnlinePayments,
		&paymentmanager.ReconcileOnlinePaymentsArgs{TenantId: v.tenantID}, out,
		func(err error) {
			if err != nil {
				v.message.Set(err.Error())
				return
			}
			v.message.Set(lang.Translate(textClosed).String() + " " + fmt.Convert(out.Closed).String())
			v.reload()
		})
}

// show pinta el detalle del pago id: sus devoluciones y, si aún queda saldo,
// el formulario para devolver.
func (v *PaymentsView) show(id string) {
	p, ok := v.presenter.Select(id).(*paymentmanager.Payment)
	if !ok {
		return
	}
	v.selected = p
	v.refundAmount.Set(fmt.Convert(p.Amount - p.RefundedAmount).String())
	v.refundReason.Set("")
	v.refundMethod = paymentmanager.MethodCash
	refunds := &paymentmanager.RefundList{}
	v.caller.Call(paymentmanager.ModelName+"."+paymentmanager.OpListRefunds,
		&paymentmanager.ListRefundsArgs{TenantId: v.tenantID, PaymentId: p.Id}, refunds,
		func(err error) {
			if err != nil {
				v.message.Set(err.Error())
				return
			}
			nodes := []*Element{H3().Text(p.Item().Label), P().Text(p.Item().Description)}
			if len(*refunds) > 0 {
				items := Ul()
				for _, r := range *refunds {
					items.Child(Li().Text(paymentmanager.FormatAmount(r.Amount, p.Currency) + " · " + r.Reason))
				}
				nodes = append(nodes, H3().Text(lang.Translate(textRefunds).String()), items)
			}
			if (p.Status == paymentmanager.StatusPaid || p.Status == paymentmanager.StatusPartiallyRefunded) && p.Amount > p.RefundedAmount {
				nodes = append(nodes, v.refundForm(p))
			}
			v.detail.Set(nodes)
		})
}

func (v *PaymentsView) refundForm(p *paymentmanager.Payment) *Element {
	form := Div().Set(clsPayRefund.AsAttr()).Child(
		Label().Text(lang.Translate(textAmount).String()).Child(Input("number").Attr("name", "refund-amount").Bind(v.refundAmount)),
		Label().Text(lang.Translate(textReason).String()).Child(NewElement("textarea").Attr("name", "refund-reason").Bind(v.refundReason)),
	)
	if p.Channel == paymentmanager.ChannelCounter {
		form.Child((&segmentedcontrol.SegmentedControl{
			Options: []segmentedcontrol.Option{
				{Value: paymentmanager.MethodCash, Label: textCash},
				{Value: paymentmanager.MethodDebit, Label: textDebit},
				{Value: paymentmanager.MethodCredit, Label: textCredit},
				{Value: paymentmanager.MethodTransfer, Label: textTransfer},
			},
			Selected: v.refundMethod,
			OnChange: func(m string) { v.refundMethod = m },
		}).Render())
	}
	return form.Child(Button().Attr("type", "button").Attr("name", "refund").
		Text(lang.Translate(textRefund).String()).
		OnClick(func(Event) { v.refund() }))
}

func (v *PaymentsView) refund() {
	amount, err := fmt.Convert(v.refundAmount.Get()).Int64()
	if err != nil {
		v.message.Set(lang.Translate(textInvalidAmount).String())
		return
	}
	out := &paymentmanager.Refund{}
	args := &paymentmanager.RefundPaymentArgs{
		TenantId:  v.tenantID,
		Id:        v.ids.NewID(),
		PaymentId: v.selected.Id,
		Amount:    amount,
		Reason:    v.refundReason.Get(),
		Method:    v.refundMethod,
	}
	v.caller.Call(paymentmanager.ModelName+"."+paymentmanager.OpRefundPayment, args, out, func(err error) {
		if err != nil {
			v.message.Set(err.Error())
			return
		}
		v.message.Set("")
		v.presenter.Reload(func(err error) {
			if err != nil {
				v.message.Set(err.Error())
				return
			}
			v.list.SetItems(v.presenter.Items())
			v.show(args.PaymentId)
		})
	})
}
