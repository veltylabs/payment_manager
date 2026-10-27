package paymentmanager

import (
	"webtyp.com/input"
	"webtyp.com/model"
)

const ModelName = "payment_manager"

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
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "reference_module", Type: model.Text(), NotNull: true, Row: model.RowDetail, Permitted: model.Permitted{Maximum: 60}},
		{Name: "reference_id", Type: model.Text(), NotNull: true, Row: model.RowDetail, Permitted: model.Permitted{Maximum: 64}},
		{Name: "channel", Type: model.Text(), NotNull: true},
		{Name: "method", Type: model.Text(), NotNull: true, Row: model.RowDetail},
		{Name: "amount", Type: model.Int(), NotNull: true, Row: model.RowTitle},
		{Name: "currency", Type: model.Text(), NotNull: true, Row: model.RowDetail, Permitted: model.Permitted{Letters: true, Minimum: 3, Maximum: 3}},
		{Name: "status", Type: model.Text(), NotNull: true, Row: model.RowDetail},
		{Name: "refunded_amount", Type: model.Int()},
		{Name: "provider_token", Type: model.Text(), OmitEmpty: true},
		{Name: "provider", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'_', '-'}, Maximum: 30}},
		// Lo que devuelve la pasarela se valida antes de guardarse: código de
		// autorización hasta 6 caracteres (Webpay) y últimos 4 dígitos de la tarjeta.
		{Name: "authorization_code", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Letters: true, Numbers: true, Maximum: 6}},
		{Name: "payment_type_code", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Letters: true, Maximum: 2}},
		{Name: "installments", Type: model.Int(), OmitEmpty: true},
		{Name: "card_last4", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Numbers: true, Maximum: 4}},
		{Name: "receipt_number", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Maximum: 40}},
		{Name: "return_to", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'_', '-'}, Maximum: 60}},
		{Name: "created_by", Type: model.Text(), OmitEmpty: true},
		{Name: "created_at", Type: model.UnixTime()},
		{Name: "paid_at", Type: model.UnixTime(), OmitEmpty: true},
		{Name: "updated_at", Type: model.UnixTime()},
		{Name: "revision", Type: model.Int()},
	},
}

var RefundModel = model.Definition{
	Name: "refund",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'.', '-', '_'}, Minimum: 1, Maximum: 26}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "payment_id", Type: model.Text(), Ref: &PaymentModel, DB: &model.FieldDB{RefColumn: "id"}, NotNull: true},
		{Name: "amount", Type: model.Int(), NotNull: true, Row: model.RowTitle},
		{Name: "kind", Type: model.Text(), NotNull: true},
		{Name: "method", Type: model.Text(), OmitEmpty: true},
		{Name: "reason", Type: input.Textarea(), NotNull: true, Permitted: model.Permitted{Maximum: 255}},
		{Name: "authorization_code", Type: model.Text(), OmitEmpty: true},
		{Name: "created_by", Type: model.Text(), OmitEmpty: true},
		{Name: "created_at", Type: model.UnixTime()},
	},
}

var CreateCounterPaymentArgsModel = model.Definition{
	Name: "create_counter_payment_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "id", Type: model.Text(), NotNull: true},
		{Name: "reference_module", Type: model.Text(), NotNull: true},
		{Name: "reference_id", Type: model.Text(), NotNull: true},
		{Name: "method", Type: input.Radio(), NotNull: true},
		{Name: "created_by", Type: model.Text()},
	},
}

var PaymentCounterModel = model.Definition{
	Name: "payment_counter",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), DB: &model.FieldDB{PK: true}, NotNull: true},
		{Name: "next_number", Type: model.Int(), NotNull: true},
	},
}

var StartOnlinePaymentArgsModel = model.Definition{
	Name: "start_online_payment_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "id", Type: model.Text(), NotNull: true},
		{Name: "reference_module", Type: model.Text(), NotNull: true},
		{Name: "reference_id", Type: model.Text(), NotNull: true},
		// return_to es la pantalla (hash) a la que vuelve el navegador; opcional.
		{Name: "return_to", Type: model.Text(), Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'_', '-'}, Maximum: 60}},
		{Name: "payer_name", Type: model.Text(), Permitted: model.Permitted{Letters: true, Tilde: true, Spaces: true, Maximum: 120}},
		{Name: "payer_email", Type: model.Text(), Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'@', '.', '-', '_', '+'}, Maximum: 120}},
		{Name: "payer_phone", Type: model.Text(), Permitted: model.Permitted{Numbers: true, Extra: []rune{'+'}, Maximum: 20}},
		{Name: "created_by", Type: model.Text()},
	},
}

var StartOnlinePaymentResultModel = model.Definition{
	Name: "start_online_payment_result",
	Fields: model.Fields{
		{Name: "payment_id", Type: model.Text()},
		{Name: "redirect_url", Type: model.Text()},
		{Name: "token", Type: model.Text()},
		{Name: "token_field", Type: model.Text()},
	},
}

var GetPaymentArgsModel = model.Definition{
	Name: "get_payment_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "id", Type: model.Text(), NotNull: true},
	},
}

var PaymentStatusResultModel = model.Definition{
	Name: "payment_status_result",
	Fields: model.Fields{
		{Name: "id", Type: model.Text()},
		{Name: "status", Type: model.Text()},
	},
}

var GetChargeArgsModel = model.Definition{
	Name: "get_charge_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "reference_module", Type: model.Text(), NotNull: true},
		{Name: "reference_id", Type: model.Text(), NotNull: true},
	},
}

var ChargeResultModel = model.Definition{
	Name: "charge_result",
	Fields: model.Fields{
		{Name: "amount", Type: model.Int()},
		{Name: "currency", Type: model.Text()},
		{Name: "payable", Type: model.Bool()},
		{Name: "terminal", Type: model.Bool()},
	},
}

var StartTerminalPaymentArgsModel = model.Definition{
	Name: "start_terminal_payment_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "id", Type: model.Text(), NotNull: true},
		{Name: "reference_module", Type: model.Text(), NotNull: true},
		{Name: "reference_id", Type: model.Text(), NotNull: true},
		{Name: "method", Type: input.Radio(), NotNull: true}, // MethodDebit | MethodCredit
		{Name: "created_by", Type: model.Text()},
	},
}

var ListPaymentsByReferenceArgsModel = model.Definition{
	Name: "list_payments_by_reference_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "reference_module", Type: model.Text(), NotNull: true},
		{Name: "reference_id", Type: model.Text(), NotNull: true},
	},
}

var ListPaymentsArgsModel = model.Definition{
	Name: "list_payments_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "from", Type: model.Int(), OmitEmpty: true},
		{Name: "to", Type: model.Int(), OmitEmpty: true},
	},
}

var RefundPaymentArgsModel = model.Definition{
	Name: "refund_payment_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "id", Type: model.Text(), NotNull: true},
		{Name: "payment_id", Type: model.Text(), NotNull: true},
		{Name: "amount", Type: model.Int(), NotNull: true, Row: model.RowTitle},
		{Name: "reason", Type: input.Textarea(), NotNull: true, Permitted: model.Permitted{Maximum: 255}},
		{Name: "method", Type: model.Text()},
		{Name: "created_by", Type: model.Text()},
	},
}

var ListRefundsArgsModel = model.Definition{
	Name: "list_refunds_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "payment_id", Type: model.Text(), NotNull: true},
	},
}

var ReconcilePaymentsArgsModel = model.Definition{
	Name: "reconcile_payments_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
	},
}

var ReconcileResultModel = model.Definition{
	Name: "reconcile_result",
	Fields: model.Fields{
		{Name: "closed", Type: model.Int()},
	},
}
