package tests

import (
	"testing"

	pm "github.com/veltylabs/payment_manager"
)

func TestPaidAmount_Table(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "paid", "r-paid")
	e.paidCounter(t, "part", "r-part")
	if _, err := e.m.RefundPayment(refundArgs("f1", "part", 5000)); err != nil {
		t.Fatal(err)
	}
	e.started(t, "pending", "r-pending")

	cases := []struct {
		name, tenant, id, ref string
		amount                int64
		err                   error
	}{
		{"paid", tenant, "paid", "r-paid", 25000, nil},
		{"partially refunded returns the net", tenant, "part", "r-part", 20000, nil},
		{"pending", tenant, "pending", "r-pending", 0, pm.ErrNotPaid},
		{"other reference", tenant, "paid", "r-part", 0, pm.ErrNotFound},
		{"other tenant", "t2", "paid", "r-paid", 0, pm.ErrNotFound},
	}
	for _, c := range cases {
		amount, currency, err := e.m.PaidAmount(c.tenant, c.id, refModule, c.ref)
		if err != c.err || amount != c.amount || (c.err == nil && currency != pm.CurrencyCLP) {
			t.Errorf("%s: got %d %s %v; want %d %v", c.name, amount, currency, err, c.amount, c.err)
		}
	}
}
