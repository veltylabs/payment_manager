package tests

import (
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
	"github.com/veltylabs/payment_manager/seed"
	"webtyp.com/orm"
	"webtyp.com/storage/mem"
)

func TestSeed_LoadsPaidAndRefunded(t *testing.T) {
	db := orm.New(mem.New())
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	refs := []seed.Ref{
		{Module: "mod", Id: "ref1"},
		{Module: "mod", Id: "ref2"},
	}

	data, err := seed.Load(m, "t1", refs)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Payments) != 2 {
		t.Errorf("expected 2 payments")
	}
	if len(data.Refunds) != 1 {
		t.Errorf("expected 1 refund")
	}
}
