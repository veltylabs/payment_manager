package paymentmanager

import (
	"webtyp.com/input"
	"webtyp.com/model"
)

const ModelName = "payment_manager"

const (
	ChannelCounter = "counter"
	ChannelOnline  = "online"
)

const (
	MethodCash     = "cash"
	MethodDebit    = "debit"
	MethodCredit   = "credit"
	MethodTransfer = "transfer"
	MethodWebpay   = "webpay"
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
	OnlinePendingTimeout = 600       // un pago online PENDING más viejo que esto se concilia
	GatewayStatusWindow  = 7 * 86400 // la pasarela guarda el estado 7 días
)

var PaymentModel = model.Definition{
	Name: "payment",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'.', '-', '_'}, Minimum: 1, Maximum: 26}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "reference_module", Type: model.Text(), NotNull: true, Permitted: model.Permitted{Maximum: 60}},
		{Name: "reference_id", Type: model.Text(), NotNull: true, Permitted: model.Permitted{Maximum: 64}},
		{Name: "channel", Type: model.Text(), NotNull: true},
		{Name: "method", Type: model.Text(), NotNull: true},
		{Name: "amount", Type: model.Int(), NotNull: true},
		{Name: "currency", Type: model.Text(), NotNull: true, Permitted: model.Permitted{Letters: true, Minimum: 3, Maximum: 3}},
		{Name: "status", Type: model.Text(), NotNull: true},
		{Name: "refunded_amount", Type: model.Int()},
		{Name: "provider_token", Type: model.Text(), OmitEmpty: true},
		{Name: "authorization_code", Type: model.Text(), OmitEmpty: true},
		{Name: "payment_type_code", Type: model.Text(), OmitEmpty: true},
		{Name: "installments", Type: model.Int(), OmitEmpty: true},
		{Name: "card_last4", Type: model.Text(), OmitEmpty: true},
		{Name: "receipt_number", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Maximum: 40}},
		{Name: "return_to", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'_', '-'}, Maximum: 60}},
		{Name: "created_by", Type: model.Text(), OmitEmpty: true},
		{Name: "created_at", Type: model.Int()},
		{Name: "paid_at", Type: model.Int(), OmitEmpty: true},
		{Name: "updated_at", Type: model.Int()},
		{Name: "revision", Type: model.Int()},
	},
}


var RefundModel = model.Definition{
	Name: "refund",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'.', '-', '_'}, Minimum: 1, Maximum: 26}},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "payment_id", Type: model.Text(), Ref: &PaymentModel, DB: &model.FieldDB{RefColumn: "id"}, NotNull: true},
		{Name: "amount", Type: model.Int(), NotNull: true},
		{Name: "kind", Type: model.Text(), NotNull: true},
		{Name: "method", Type: model.Text(), OmitEmpty: true},
		{Name: "reason", Type: input.Textarea(), NotNull: true, Permitted: model.Permitted{Maximum: 255}},
		{Name: "authorization_code", Type: model.Text(), OmitEmpty: true},
		{Name: "created_by", Type: model.Text(), OmitEmpty: true},
		{Name: "created_at", Type: model.Int()},
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
		{Name: "receipt_number", Type: model.Text()},
	},
}

var StartOnlinePaymentArgsModel = model.Definition{
	Name: "start_online_payment_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "id", Type: model.Text(), NotNull: true},
		{Name: "reference_module", Type: model.Text(), NotNull: true},
		{Name: "reference_id", Type: model.Text(), NotNull: true},
		{Name: "return_to", Type: model.Text(), NotNull: true},
	},
}

var StartOnlinePaymentResultModel = model.Definition{
	Name: "start_online_payment_result",
	Fields: model.Fields{
		{Name: "payment_id", Type: model.Text(), NotNull: true},
		{Name: "redirect_url", Type: model.Text(), NotNull: true},
		{Name: "token", Type: model.Text(), NotNull: true},
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
		{Name: "id", Type: model.Text(), NotNull: true},
		{Name: "status", Type: model.Text(), NotNull: true},
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
		{Name: "amount", Type: model.Int(), NotNull: true},
		{Name: "currency", Type: model.Text(), NotNull: true},
		{Name: "payable", Type: model.Bool(), NotNull: true},
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
		{Name: "from", Type: model.Int(), NotNull: true},
		{Name: "to", Type: model.Int(), NotNull: true},
	},
}

var RefundPaymentArgsModel = model.Definition{
	Name: "refund_payment_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "id", Type: model.Text(), NotNull: true},
		{Name: "payment_id", Type: model.Text(), NotNull: true},
		{Name: "amount", Type: model.Int(), NotNull: true},
		{Name: "reason", Type: input.Textarea(), NotNull: true, Permitted: model.Permitted{Maximum: 255}},
		{Name: "method", Type: model.Text()},
	},
}

var ListRefundsArgsModel = model.Definition{
	Name: "list_refunds_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "payment_id", Type: model.Text(), NotNull: true},
	},
}

var ReconcileOnlinePaymentsArgsModel = model.Definition{
	Name: "reconcile_online_payments_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
	},
}

var ReconcileResultModel = model.Definition{
	Name: "reconcile_result",
	Fields: model.Fields{
		{Name: "closed", Type: model.Int(), NotNull: true},
	},
}
