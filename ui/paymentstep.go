package ui

import (
	paymentmanager "github.com/veltylabs/payment_manager"
	. "webtyp.com/dom"
	. "webtyp.com/html"
	"webtyp.com/router"
	"webtyp.com/components/segmentedcontrol"
	"webtyp.com/fmt"
)

type PaymentStep struct {
	Element
	Caller    router.Caller
	TenantId  string
	RefModule string
	RefId     string
	OnPaid    func()

	amount    *SignalString
	errorMsg  *SignalString
	nodes     *SignalNodes
}

func NewPaymentStep(caller router.Caller, tenantId, refModule, refId string, onPaid func()) *PaymentStep {
	v := &PaymentStep{
		Caller:    caller,
		TenantId:  tenantId,
		RefModule: refModule,
		RefId:     refId,
		OnPaid:    onPaid,
		amount:    NewString("Cargando..."),
		errorMsg:  NewString(""),
		nodes:     NewNodes(),
	}
	v.Element = *Div().
		Child(
			Div().BindText(v.errorMsg).Attr("class", "error"),
			H2().BindText(v.amount),
			Div().BindChildren(v.nodes),
		)
	v.loadCharge()
	return v
}

func (v *PaymentStep) Render() *Element {
	return &v.Element
}

func (v *PaymentStep) loadCharge() {
	out := &paymentmanager.ChargeResult{}
	args := paymentmanager.GetChargeArgs{
		TenantId:        v.TenantId,
		ReferenceModule: v.RefModule,
		ReferenceId:     v.RefId,
	}

	v.Caller.Call(paymentmanager.ModelName+"."+paymentmanager.OpGetCharge, &args, out, func(err error) {
		if err != nil {
			v.errorMsg.Set("Error: " + err.Error())
			v.amount.Set("")
			return
		}

		if out.Amount == 0 {
			v.amount.Set("Nada que pagar")
			if v.OnPaid != nil {
				v.OnPaid()
			}
			return
		}

		v.amount.Set("Total a pagar: " + paymentmanager.FormatAmount(out.Amount, out.Currency))
		v.renderMethods()
	})
}

func (v *PaymentStep) renderMethods() {
	method := paymentmanager.MethodCash
	ctrl := segmentedcontrol.SegmentedControl{
		Options: []segmentedcontrol.Option{
			{Value: paymentmanager.MethodCash, Label: "Efectivo"},
			{Value: "CARD", Label: "Tarjeta (Máquina)"},
			{Value: "WEBPAY", Label: "Webpay"},
		},
		Selected: method,
		OnChange: func(val string) { method = val },
	}

	btn := Button().Text("Pagar").OnClick(func(_ Event) {
		if method == "WEBPAY" {
			v.startOnlinePayment()
		} else {
			v.createCounterPayment(method)
		}
	})

	v.nodes.Set([]*Element{
		ctrl.Render(),
		btn,
	})
}

func (v *PaymentStep) createCounterPayment(method string) {
	out := &paymentmanager.Payment{}
	args := paymentmanager.CreateCounterPaymentArgs{
		TenantId:        v.TenantId,
		Id:              "tmp",
		ReferenceModule: v.RefModule,
		ReferenceId:     v.RefId,
		Method:          method,
	}

	v.Caller.Call(paymentmanager.ModelName+"."+paymentmanager.OpCreateCounterPayment, &args, out, func(err error) {
		if err != nil {
			v.errorMsg.Set("Error al registrar pago: " + err.Error())
			return
		}
		if v.OnPaid != nil {
			v.OnPaid()
		}
	})
}


func (v *PaymentStep) startOnlinePayment() {
	out := &paymentmanager.GatewayStarted{}
	args := paymentmanager.StartOnlinePaymentArgs{
		TenantId:        v.TenantId,
		Id:              "tmp",
		ReferenceModule: v.RefModule,
		ReferenceId:     v.RefId,
		ReturnTo:        "", // dom.URL() in wasm would be fine, mock stub for build
	}

	v.Caller.Call(paymentmanager.ModelName+"."+paymentmanager.OpStartOnlinePayment, &args, out, func(err error) {
		if err != nil {
			v.errorMsg.Set("Error al iniciar Webpay: " + err.Error())
			return
		}
		NavigatePost(out.RedirectURL, []fmt.KeyValue{{Key: "token_ws", Value: out.Token}})
	})
}
