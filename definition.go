package paymentmanager

import (
	"webtyp.com/input"
	"webtyp.com/model"
)



const (
	ChannelCounter  = "counter"
	ChannelOnline   = "online"
	ChannelTerminal = "terminal" // cobro en caja empujado a la máquina de tarjeta
)

const (
	MethodCash     = "cash"
	MethodDebit    = "debit"
	MethodCredit   = "credit"
	MethodTransfer = "transfer"
	MethodCard     = "card" // pago en línea con tarjeta, cualquiera sea el procesador
)

const (
	CurrencyCLP = "CLP"
	CurrencyUSD = "USD"
)

const (
	StatusPending           = "PENDING"
	StatusPaid              = "PAID"
	StatusFailed            = "FAILED"
	StatusCancelled         = "CANCELLED" // el cliente abandonó el pago online
	StatusExpired           = "EXPIRED"   // venció sin respuesta de la pasarela
	StatusPartiallyRefunded = "PARTIALLY_REFUNDED"
	StatusRefunded          = "REFUNDED"
)

const (
	RefundKindReversed  = "REVERSED"  // pasarela: reversa total
	RefundKindNullified = "NULLIFIED" // pasarela: anulación total o parcial
	RefundKindManual    = "MANUAL"    // mesón: devolución registrada por el cajero
)

// Tiempos (segundos).
const (
	PendingTimeout      = 600       // un pago PENDING (online o máquina) más viejo que esto se concilia
	GatewayStatusWindow = 7 * 86400 // la pasarela guarda el estado 7 días
)

var PaymentModel = model.Definition{
	Name: "payment",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'.', '-', '_'}, Minimum: 1, Maximum: 26}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true, ServerOnly: true},
		{Name: "reference_module", Type: model.Text(), NotNull: true, Item: model.ItemDescription, Permitted: model.Permitted{Maximum: 60}},
		{Name: "reference_id", Type: model.Text(), NotNull: true, Item: model.ItemDescription, Permitted: model.Permitted{Maximum: 64}},
		{Name: "channel", Type: model.Text(), NotNull: true},
		{Name: "method", Type: model.Text(), NotNull: true, Item: model.ItemDescription},
		{Name: "amount", Type: model.Int(), NotNull: true, Item: model.ItemLabel, ServerOnly: true},
		{Name: "currency", Type: model.Text(), NotNull: true, Item: model.ItemDescription, Permitted: model.Permitted{Letters: true, Minimum: 3, Maximum: 3}},
		{Name: "status", Type: model.Text(), NotNull: true, Item: model.ItemDescription, ServerOnly: true},
		{Name: "refunded_amount", Type: model.Int()},
		{Name: "provider_token", Type: model.Text(), OmitEmpty: true, ServerOnly: true},
		{Name: "provider", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'_', '-'}, Maximum: 30}, ServerOnly: true},
		// Lo que devuelve la pasarela se valida antes de guardarse: código de
		// autorización hasta 6 caracteres (Webpay) y últimos 4 dígitos de la tarjeta.
		{Name: "authorization_code", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Letters: true, Numbers: true, Maximum: 6}},
		{Name: "payment_type_code", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Letters: true, Maximum: 2}},
		{Name: "installments", Type: model.Int(), OmitEmpty: true},
		{Name: "card_last4", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Numbers: true, Maximum: 4}},
		{Name: "receipt_number", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Maximum: 40}},
		{Name: "return_to", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'_', '-'}, Maximum: 60}},
		{Name: "created_by", Type: model.Text(), OmitEmpty: true},
		{Name: "created_at", Type: model.UnixTime(), ServerOnly: true},
		{Name: "paid_at", Type: model.UnixTime(), OmitEmpty: true},
		{Name: "updated_at", Type: model.UnixTime(), ServerOnly: true},
		{Name: "revision", Type: model.Int()},
	},
	Allow: model.Create | model.Read,
	Rules: model.Rules{
		Create: model.AddMyBusinessRule,
		Read: model.HaveNoBusinessRule,
	},
	ExternalCalls: []model.ExternalCallDefinition{
		{Name: "TerminalPush", Mode: model.WaitForReply, MaxWaitSeconds: 10},
		{Name: "TerminalStatus", Mode: model.RetryInBackground, MaxAttempts: 20},
	},
}

var RefundModel = model.Definition{
	Name: "refund",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'.', '-', '_'}, Minimum: 1, Maximum: 26}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true, ServerOnly: true},
		{Name: "payment_id", Type: model.Text(), Ref: &PaymentModel, DB: &model.FieldDB{RefColumn: "id"}, NotNull: true},
		{Name: "amount", Type: model.Int(), NotNull: true, Item: model.ItemLabel, ServerOnly: true},
		{Name: "kind", Type: model.Text(), NotNull: true},
		{Name: "method", Type: model.Text(), OmitEmpty: true},
		{Name: "reason", Type: input.Textarea(), NotNull: true, Permitted: model.Permitted{Maximum: 255}},
		{Name: "authorization_code", Type: model.Text(), OmitEmpty: true},
		{Name: "created_by", Type: model.Text(), OmitEmpty: true},
		{Name: "created_at", Type: model.UnixTime(), ServerOnly: true},
	},
	Allow: model.Create | model.Read,
	Rules: model.Rules{
		Create: model.AddMyBusinessRule,
		Read: model.HaveNoBusinessRule,
	},
	ExternalCalls: []model.ExternalCallDefinition{
		{Name: "GatewayRefund", Mode: model.WaitForReply, MaxWaitSeconds: 15},
	},
}


var PaymentCounterModel = model.Definition{
	Name: "payment_counter",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), DB: &model.FieldDB{PK: true}, NotNull: true},
		{Name: "next_number", Type: model.Int(), NotNull: true},
	},
}




var PaymentStatusResultModel = model.Definition{
	Name: "payment_status_result",
	Fields: model.Fields{
		{Name: "id", Type: model.Text()},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "status", Type: model.Text()},
	},
	Allow: model.Read,
	Rules: model.Rules{
		Read: model.AddMyBusinessRule,
	},
}


var ChargeResultModel = model.Definition{
	Name: "charge_result",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "reference_module", Type: model.Text(), NotNull: true},
		{Name: "reference_id", Type: model.Text(), NotNull: true},
		{Name: "amount", Type: model.Int()},
		{Name: "currency", Type: model.Text()},
		{Name: "payable", Type: model.Bool()},
		{Name: "terminal", Type: model.Bool()},
	},
	Allow: model.Read,
	Rules: model.Rules{
		Read: model.AddMyBusinessRule,
	},
}








var OnlinePaymentRequestModel = model.Definition{
	Name: "payment_online_request",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'.', '-', '_'}, Minimum: 1, Maximum: 26}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true, ServerOnly: true},
		{Name: "reference_module", Type: model.Text(), NotNull: true},
		{Name: "reference_id", Type: model.Text(), NotNull: true},
		{Name: "return_url", Type: input.Text()},
		{Name: "payment_id", Type: model.Text(), ServerOnly: true},
		{Name: "redirect_url", Type: model.Text(), ServerOnly: true},
		{Name: "status", Type: model.Text(), ServerOnly: true},
		{Name: "failure", Type: model.Text(), ServerOnly: true},
		{Name: "created_at", Type: model.UnixTime(), ServerOnly: true},
		{Name: "payer_name", Type: model.Text()},
		{Name: "payer_email", Type: model.Text()},
		{Name: "payer_phone", Type: model.Text()},
		{Name: "token", Type: model.Text(), ServerOnly: true},
		{Name: "token_field", Type: model.Text(), ServerOnly: true},
	},
	Allow: model.Create | model.Read,
	Rules: model.Rules{
		Create: model.AddMyBusinessRule,
		Read: model.HaveNoBusinessRule,
	},
	ExternalCalls: []model.ExternalCallDefinition{
		{Name: "GatewayStart", Mode: model.WaitForReply, MaxWaitSeconds: 15},
	},
}

var PaymentReconciliationModel = model.Definition{
	Name: "payment_reconciliation",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'.', '-', '_'}, Minimum: 1, Maximum: 26}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true, ServerOnly: true},
		{Name: "status", Type: model.Text(), ServerOnly: true},
		{Name: "failure", Type: model.Text(), ServerOnly: true},
		{Name: "reconciled", Type: model.Int(), ServerOnly: true},
		{Name: "actor_id", Type: model.Text(), ServerOnly: true},
		{Name: "created_at", Type: model.UnixTime(), ServerOnly: true},
	},
	Allow: model.Create | model.Read,
	Rules: model.Rules{
		Create: model.AddMyBusinessRule,
		Read: model.HaveNoBusinessRule,
	},
	ExternalCalls: []model.ExternalCallDefinition{
		{Name: "Reconcile", Mode: model.RetryInBackground, MaxAttempts: 3},
	},
}

var PaymentManualConfirmationModel = model.Definition{
	Name: "payment_manual_confirmation",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'.', '-', '_'}, Minimum: 1, Maximum: 26}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true, ServerOnly: true},
		{Name: "payment_id", Type: model.Text(), NotNull: true},
		{Name: "note", Type: input.Text(), NotNull: true},
		{Name: "actor_id", Type: model.Text(), ServerOnly: true},
		{Name: "created_at", Type: model.UnixTime(), ServerOnly: true},
	},
	Allow: model.Create | model.Read,
	Rules: model.Rules{
		Create: model.AddMyBusinessRule,
		Read: model.HaveNoBusinessRule,
	},
	ExternalCalls: []model.ExternalCallDefinition{
		{Name: "Confirm", Mode: model.WaitForReply, MaxWaitSeconds: 10},
	},
}


var Definition = model.ModuleDefinition{
	Name: "payment_manager",
	Entities: []model.Definition{
		PaymentModel,
		PaymentCounterModel,
		PaymentManualConfirmationModel,
		OnlinePaymentRequestModel,
		RefundModel,
		PaymentReconciliationModel,
		ChargeResultModel,
		PaymentStatusResultModel,
	},
}
