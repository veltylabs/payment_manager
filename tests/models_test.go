package tests

import (
	"testing"

	"webtyp.com/json"
	"webtyp.com/model"

	pm "github.com/veltylabs/payment_manager"
)

// roundTrip codifica y decodifica v en una instancia nueva y compara el JSON.
func roundTrip(t *testing.T, name string, v model.Encodable, fresh model.Decodable) {
	t.Helper()
	var first, second []byte
	if err := json.Encode(v, &first); err != nil {
		t.Fatalf("%s encode: %v", name, err)
	}
	if err := json.Decode(string(first), fresh); err != nil {
		t.Fatalf("%s decode: %v", name, err)
	}
	if err := json.Encode(fresh.(model.Encodable), &second); err != nil {
		t.Fatalf("%s re-encode: %v", name, err)
	}
	if string(first) != string(second) {
		t.Fatalf("%s round trip:\n%s\n%s", name, first, second)
	}
}

func TestModels_RoundTripAndValidate(t *testing.T) {
	roundTrip(t, "payment", &pm.Payment{Id: "p1", TenantId: tenant, ReferenceModule: refModule, ReferenceId: refID, Channel: pm.ChannelOnline,
		Method: pm.MethodCard, Amount: 25000, Currency: pm.CurrencyCLP, Status: pm.StatusPaid, RefundedAmount: 10, ProviderToken: "tok",
		AuthorizationCode: "A1", PaymentTypeCode: "VD", Installments: 3, CardLast4: "1234", ReceiptNumber: "R", ReturnTo: "x",
		CreatedBy: "u", CreatedAt: 1, PaidAt: 2, UpdatedAt: 3, Revision: 4}, &pm.Payment{})
	roundTrip(t, "refund", &pm.Refund{Id: "f1", TenantId: tenant, PaymentId: "p1", Amount: 5, Kind: pm.RefundKindManual, Method: pm.MethodCash,
		Reason: "r", AuthorizationCode: "A", CreatedBy: "u", CreatedAt: 1}, &pm.Refund{})
	roundTrip(t, "create_counter_payment_args", &pm.CreateCounterPaymentArgs{TenantId: tenant, Id: "p1", ReferenceModule: refModule, ReferenceId: refID, Method: pm.MethodCash}, &pm.CreateCounterPaymentArgs{})
	roundTrip(t, "start_online_payment_args", &pm.StartOnlinePaymentArgs{TenantId: tenant, Id: "p1", ReferenceModule: refModule, ReferenceId: refID, ReturnTo: "x"}, &pm.StartOnlinePaymentArgs{})
	roundTrip(t, "start_online_payment_result", &pm.StartOnlinePaymentResult{PaymentId: "p1", RedirectUrl: "u", Token: "t"}, &pm.StartOnlinePaymentResult{})
	roundTrip(t, "get_payment_args", &pm.GetPaymentArgs{TenantId: tenant, Id: "p1"}, &pm.GetPaymentArgs{})
	roundTrip(t, "payment_status_result", &pm.PaymentStatusResult{Id: "p1", Status: pm.StatusPaid}, &pm.PaymentStatusResult{})
	roundTrip(t, "get_charge_args", &pm.GetChargeArgs{TenantId: tenant, ReferenceModule: refModule, ReferenceId: refID}, &pm.GetChargeArgs{})
	roundTrip(t, "charge_result", &pm.ChargeResult{Amount: 1, Currency: pm.CurrencyCLP, Payable: true}, &pm.ChargeResult{})
	roundTrip(t, "list_payments_by_reference_args", &pm.ListPaymentsByReferenceArgs{TenantId: tenant, ReferenceModule: refModule, ReferenceId: refID}, &pm.ListPaymentsByReferenceArgs{})
	roundTrip(t, "list_payments_args", &pm.ListPaymentsArgs{TenantId: tenant, From: 1, To: 2}, &pm.ListPaymentsArgs{})
	roundTrip(t, "refund_payment_args", &pm.RefundPaymentArgs{TenantId: tenant, Id: "f1", PaymentId: "p1", Amount: 1, Reason: "r", Method: pm.MethodCash}, &pm.RefundPaymentArgs{})
	roundTrip(t, "list_refunds_args", &pm.ListRefundsArgs{TenantId: tenant, PaymentId: "p1"}, &pm.ListRefundsArgs{})
	roundTrip(t, "reconcile_payments_args", &pm.ReconcilePaymentsArgs{TenantId: tenant}, &pm.ReconcilePaymentsArgs{})
	roundTrip(t, "reconcile_result", &pm.ReconcileResult{Closed: 2}, &pm.ReconcileResult{})

	if err := (&pm.GetPaymentArgs{TenantId: tenant, Id: "p1"}).Validate(model.ActionRead); err != nil {
		t.Errorf("valid args rejected: %v", err)
	}
	if err := (&pm.GetPaymentArgs{}).Validate(model.ActionRead); err == nil {
		t.Error("missing tenant_id accepted")
	}
}

// fielderList es la forma que ormc genera para cada lista de modelos.
type fielderList interface {
	Len() int
	At(i int) model.Fielder
	Append() model.Fielder
	IsNil() bool
	EncodeFields(w model.FieldWriter)
	DecodeFields(r model.FieldReader)
}

func TestModels_GeneratedLists(t *testing.T) {
	cases := []struct {
		name string
		list fielderList
	}{
		{"payment", &pm.PaymentList{}}, {"refund", &pm.RefundList{}},
		{"create_counter_payment_args", &pm.CreateCounterPaymentArgsList{}}, {"start_online_payment_args", &pm.StartOnlinePaymentArgsList{}},
		{"start_online_payment_result", &pm.StartOnlinePaymentResultList{}}, {"get_payment_args", &pm.GetPaymentArgsList{}},
		{"payment_status_result", &pm.PaymentStatusResultList{}}, {"get_charge_args", &pm.GetChargeArgsList{}},
		{"charge_result", &pm.ChargeResultList{}}, {"list_payments_by_reference_args", &pm.ListPaymentsByReferenceArgsList{}},
		{"list_payments_args", &pm.ListPaymentsArgsList{}}, {"refund_payment_args", &pm.RefundPaymentArgsList{}},
		{"list_refunds_args", &pm.ListRefundsArgsList{}}, {"reconcile_payments_args", &pm.ReconcilePaymentsArgsList{}},
		{"reconcile_result", &pm.ReconcileResultList{}},
	}
	for _, c := range cases {
		if c.list.IsNil() || c.list.Len() != 0 {
			t.Errorf("%s: a new list is empty and non-nil", c.name)
		}
		added := c.list.Append()
		if c.list.Len() != 1 || c.list.At(0) != added {
			t.Errorf("%s: Append then At(0) returns the appended record", c.name)
		}
		got, ok := added.(model.Model)
		if !ok || got.ModelName() != c.name {
			t.Errorf("%s: element model name = %v", c.name, added)
			continue
		}
		if len(got.Schema()) != len(got.Pointers()) {
			t.Errorf("%s: %d schema fields, %d pointers", c.name, len(got.Schema()), len(got.Pointers()))
		}
		// Encode/DecodeFields de la lista no escriben nada: el codec recorre los elementos.
		c.list.EncodeFields(nil)
		c.list.DecodeFields(nil)
		if c.list.Len() != 1 {
			t.Errorf("%s: list Encode/DecodeFields must not change it", c.name)
		}
	}
}

// Los modelos de resultado se validan con lo que el servidor les pone.
func TestModels_ResultsValidate(t *testing.T) {
	for _, c := range []struct {
		name string
		v    interface{ Validate(byte) error }
	}{
		{"start_online_payment_result", &pm.StartOnlinePaymentResult{PaymentId: "p1", RedirectUrl: "https://x", Token: "t"}},
		{"payment_status_result", &pm.PaymentStatusResult{Id: "p1", Status: pm.StatusPaid}},
		{"charge_result", &pm.ChargeResult{Amount: 1, Currency: pm.CurrencyCLP}},
		{"reconcile_result", &pm.ReconcileResult{Closed: 1}},
	} {
		if err := c.v.Validate(model.ActionRead); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if ext := (&pm.Refund{}).SchemaExt(); len(ext) != 1 || ext[0].Ref != pm.PaymentModel.Name {
		t.Errorf("refund.payment_id references payment: %+v", ext)
	}
}

func TestRowsDeclaredOnFields(t *testing.T) {
	p := pm.Payment{
		Id: "p1",
		Amount: 25000,
		Method: pm.MethodCredit,
		Status: pm.StatusPaid,
		ReferenceModule: "booking",
		ReferenceId: "ref1",
		Currency: pm.CurrencyCLP,
	}
	r := model.RowOf(&p)
	if r.Title != "25000" {
		t.Errorf("Title = %q, want 25000", r.Title)
	}

}
