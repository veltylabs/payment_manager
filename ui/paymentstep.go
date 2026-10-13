package ui

import (
	"webtyp.com/components/segmentedcontrol"
	"webtyp.com/fmt"
	"webtyp.com/lang"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/widget"

	. "webtyp.com/dom"
	. "webtyp.com/html"

	paymentmanager "github.com/veltylabs/payment_manager"
)

// NamePaymentStep es la identidad de widget del paso de pago.
const NamePaymentStep = widget.Name("paymentstep")

const (
	PartAmount  = widget.Part("amount")
	PartError   = widget.Part("error")
	PartBody    = widget.Part("body")
	PartReceipt = widget.Part("receipt")
	PartPay     = widget.Part("pay")
)

var (
	clsStepRoot    = NamePaymentStep.Root()
	clsStepAmount  = NamePaymentStep.Class(PartAmount)
	clsStepError   = NamePaymentStep.Class(PartError)
	clsStepBody    = NamePaymentStep.Class(PartBody)
	clsStepReceipt = NamePaymentStep.Class(PartReceipt)
	clsStepPay     = NamePaymentStep.Class(PartPay)
)

// Textos fijos (claves en inglés; su español en lang.json).
const (
	textLoading       lang.Text = "Loading…"
	textNothingToPay  lang.Text = "Nothing to pay"
	textCash          lang.Text = "Cash"
	textDebit         lang.Text = "Debit"
	textCredit        lang.Text = "Credit"
	textTransfer      lang.Text = "Transfer"
	textReceiptNumber lang.Text = "Receipt number"
	textRegisterPay   lang.Text = "Register payment"
	textPayWithWebpay lang.Text = "Pay with Webpay"
)

// FieldToken es el campo del formulario con que la pasarela recibe el token.
const FieldToken = "token_ws"

// PaymentStep cobra una referencia. En mesón registra el pago con su medio; en
// online lleva al cliente a la pasarela. Lo incrusta, por ejemplo, el paso 6 de
// la reserva de appointment_booking.
type PaymentStep struct {
	Element // value embed — NEVER pointer

	Caller          router.Caller
	IDs             model.IDGenerator
	TenantID        string
	Channel         string // paymentmanager.ChannelCounter | paymentmanager.ChannelOnline
	ReferenceModule string
	ReferenceID     string
	ReturnTo        string                 // online: id de la pantalla (hash) a la que se vuelve
	OnPaid          func(paymentID string) // mesón: después de registrar el pago

	amount  *SignalString
	errMsg  *SignalString
	body    *SignalNodes
	busy    *SignalBool
	method  string
	receipt *SignalString
}

func (s *PaymentStep) WidgetName() widget.Name { return NamePaymentStep }
func (s *PaymentStep) WidgetKind() widget.Kind { return widget.Form }

// Init prepara las señales y pide el monto. Llamarla de nuevo no hace nada.
func (s *PaymentStep) Init(_ Ctx) {
	if s.amount != nil {
		return
	}
	s.amount = NewString(lang.Translate(textLoading).String())
	s.errMsg = NewString("")
	s.body = NewNodes()
	s.busy = NewBool(false)
	s.receipt = NewString("")
	s.method = paymentmanager.MethodCash
	s.loadCharge()
}

func (s *PaymentStep) Render() *Element {
	s.Init(nil)
	return Div().Set(clsStepRoot.AsAttr()).Child(
		P().Set(clsStepAmount.AsAttr()).BindText(s.amount),
		P().Set(clsStepError.AsAttr()).Attr("role", "alert").BindText(s.errMsg),
		Div().Set(clsStepBody.AsAttr()).BindChildren(s.body),
	)
}

func (s *PaymentStep) loadCharge() {
	out := &paymentmanager.ChargeResult{}
	s.Caller.Call(paymentmanager.ModelName+"."+paymentmanager.OpGetCharge,
		&paymentmanager.GetChargeArgs{TenantId: s.TenantID, ReferenceModule: s.ReferenceModule, ReferenceId: s.ReferenceID}, out,
		func(err error) {
			if err != nil {
				s.amount.Set("")
				s.errMsg.Set(err.Error())
				return
			}
			if !out.Payable {
				s.amount.Set(lang.Translate(textNothingToPay).String())
				return
			}
			s.amount.Set(paymentmanager.FormatAmount(out.Amount, out.Currency))
			if s.Channel == paymentmanager.ChannelOnline {
				s.body.Set([]*Element{s.payButton(textPayWithWebpay, s.startOnline)})
				return
			}
			s.body.Set([]*Element{s.methodPicker(), s.receiptField(), s.payButton(textRegisterPay, s.registerCounter)})
		})
}

func (s *PaymentStep) methodPicker() *Element {
	picker := &segmentedcontrol.SegmentedControl{
		Options: []segmentedcontrol.Option{
			{Value: paymentmanager.MethodCash, Label: textCash},
			{Value: paymentmanager.MethodDebit, Label: textDebit},
			{Value: paymentmanager.MethodCredit, Label: textCredit},
			{Value: paymentmanager.MethodTransfer, Label: textTransfer},
		},
		Selected: s.method,
		OnChange: func(v string) { s.method = v },
	}
	return picker.Render()
}

func (s *PaymentStep) receiptField() *Element {
	return Label().Set(clsStepReceipt.AsAttr()).
		Text(lang.Translate(textReceiptNumber).String()).
		Child(Input("text").Attr("name", "receipt").Bind(s.receipt))
}

func (s *PaymentStep) payButton(label lang.Text, run func()) *Element {
	return Button().Set(clsStepPay.AsAttr()).
		Attr("type", "button").
		BindState(widget.Busy, s.busy).
		Text(lang.Translate(label).String()).
		OnClick(func(Event) {
			if s.busy.Get() {
				return
			}
			s.busy.Set(true)
			s.errMsg.Set("")
			run()
		})
}

func (s *PaymentStep) registerCounter() {
	out := &paymentmanager.Payment{}
	args := &paymentmanager.CreateCounterPaymentArgs{
		TenantId:        s.TenantID,
		Id:              s.IDs.NewID(),
		ReferenceModule: s.ReferenceModule,
		ReferenceId:     s.ReferenceID,
		Method:          s.method,
		ReceiptNumber:   s.receipt.Get(),
	}
	s.Caller.Call(paymentmanager.ModelName+"."+paymentmanager.OpCreateCounterPayment, args, out, func(err error) {
		s.busy.Set(false)
		if err != nil {
			s.errMsg.Set(err.Error())
			return
		}
		if s.OnPaid != nil {
			s.OnPaid(out.Id)
		}
	})
}

func (s *PaymentStep) startOnline() {
	out := &paymentmanager.StartOnlinePaymentResult{}
	args := &paymentmanager.StartOnlinePaymentArgs{
		TenantId:        s.TenantID,
		Id:              s.IDs.NewID(),
		ReferenceModule: s.ReferenceModule,
		ReferenceId:     s.ReferenceID,
		ReturnTo:        s.ReturnTo,
	}
	s.Caller.Call(paymentmanager.ModelName+"."+paymentmanager.OpStartOnlinePayment, args, out, func(err error) {
		if err != nil {
			s.busy.Set(false)
			s.errMsg.Set(err.Error())
			return
		}
		NavigatePost(out.RedirectUrl, []fmt.KeyValue{{Key: FieldToken, Value: out.Token}})
	})
}
