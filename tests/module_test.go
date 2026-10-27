package tests

import (
	"testing"

	"webtyp.com/orm"
	"webtyp.com/storage/mem"

	pm "github.com/veltylabs/payment_manager"
)

func TestNew_RequiresCharges(t *testing.T) {
	if _, err := pm.New(orm.New(mem.New()), pm.Deps{}); err == nil {
		t.Fatal("New without Charges must fail")
	}
}

func TestNew_WithoutPublisherDoesNotPublish(t *testing.T) {
	m, err := pm.New(orm.New(mem.New()), pm.Deps{Charges: []pm.ChargeSource{{Module: refModule, Reader: &fakeCharge{amount: 1, currency: pm.CurrencyCLP, payable: true}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.CreateCounterPayment(pm.CreateCounterPaymentArgs{TenantId: tenant, Id: "p1", ReferenceModule: refModule, ReferenceId: refID, Method: pm.MethodCash}); err != nil {
		t.Fatalf("a nil Publisher must not break a write: %v", err)
	}
}
