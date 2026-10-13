package tests

import (
	"testing"

	pm "github.com/veltylabs/payment_manager"
)

func TestCharge_UnknownModule(t *testing.T) {
	_, _, _, err := newEnv(t).m.Charge(tenant, "otro_modulo", refID)
	wantErr(t, err, pm.ErrUnknownReference)
}

func TestCharge_ReaderError(t *testing.T) {
	e := newEnv(t)
	e.charge.err = errSimulated
	_, _, _, err := e.m.Charge(tenant, refModule, refID)
	if err != errSimulated {
		t.Fatalf("err = %v", err)
	}
}

func TestCharge_UnsupportedCurrency(t *testing.T) {
	e := newEnv(t)
	e.charge.currency = "EUR"
	_, _, _, err := e.m.Charge(tenant, refModule, refID)
	wantErr(t, err, pm.ErrUnsupportedCurrency)
}

func TestCharge_OK(t *testing.T) {
	amount, currency, payable, err := newEnv(t).m.Charge(tenant, refModule, refID)
	if err != nil || amount != 25000 || currency != pm.CurrencyCLP || !payable {
		t.Fatalf("got %d %s %v %v", amount, currency, payable, err)
	}
}
