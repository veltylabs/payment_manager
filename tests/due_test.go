package tests

import (
	"testing"

	pm "github.com/veltylabs/payment_manager"
)

func TestDue_BalanceCases(t *testing.T) {
	cases := []struct {
		name    string
		amount  int64
		payable bool
		setup   func(e *env)
		wantErr error
		wantDue int64
	}{
		{
			"30000, true, none -> 30000",
			30000, true,
			func(e *env) {},
			nil, 30000,
		},
		{
			"30000, true, PAID 10000 -> 20000",
			30000, true,
			func(e *env) {
				e.charge.amount = 10000
				e.paidCounter(t, "p1", refID)
			},
			nil, 20000,
		},
		{
			"30000, true, PAID 30000 -> ErrAlreadyPaid",
			30000, true,
			func(e *env) {
				e.charge.amount = 30000
				e.paidCounter(t, "p1", refID)
			},
			pm.ErrAlreadyPaid, 0,
		},
		{
			"30000, true, PARTIALLY_REFUNDED 30000 (5000 refunded) -> ErrAlreadyPaid",
			30000, true,
			func(e *env) {
				e.charge.amount = 30000
				e.paidCounter(t, "p1", refID)
				_, err := e.m.RefundPayment(pm.RefundPaymentArgs{
					TenantId: tenant, Id: "f1", PaymentId: "p1", Amount: 5000, Reason: "reason", Method: pm.MethodCash,
				})
				if err != nil {
					t.Fatalf("RefundPayment: %v", err)
				}
			},
			pm.ErrAlreadyPaid, 0,
		},
		{
			"30000, true, REFUNDED 30000 -> 30000",
			30000, true,
			func(e *env) {
				e.charge.amount = 30000
				e.paidCounter(t, "p1", refID)
				_, err := e.m.RefundPayment(pm.RefundPaymentArgs{
					TenantId: tenant, Id: "f1", PaymentId: "p1", Amount: 30000, Reason: "reason", Method: pm.MethodCash,
				})
				if err != nil {
					t.Fatalf("RefundPayment: %v", err)
				}
			},
			nil, 30000,
		},
		{
			"30000, true, PENDING 30000 (online) -> 30000",
			30000, true,
			func(e *env) {
				e.charge.amount = 30000
				e.started(t, "p1", refID)
			},
			nil, 30000,
		},
		{
			"30000, false, PAID 30000 -> ErrAlreadyPaid",
			30000, false,
			func(e *env) {
				e.charge.amount = 30000
				e.charge.payable = true
				e.paidCounter(t, "p1", refID)
			},
			pm.ErrAlreadyPaid, 0,
		},
		{
			"0, false, none -> ErrNotPayable",
			0, false,
			func(e *env) {},
			pm.ErrNotPayable, 0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			c.setup(e)
			e.charge.amount = c.amount
			e.charge.payable = c.payable

			amount, currency, err := e.m.Due(tenant, refModule, refID)
			if err != c.wantErr {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
			if err == nil {
				if amount != c.wantDue {
					t.Errorf("amount = %d, want %d", amount, c.wantDue)
				}
				if currency != pm.CurrencyCLP {
					t.Errorf("currency = %s, want %s", currency, pm.CurrencyCLP)
				}
			}
		})
	}
}

func TestDue_PartialPayment(t *testing.T) {
	e := newEnv(t)
	e.charge.amount = 10000
	e.paidCounter(t, "p1", refID)

	e.charge.amount = 30000
	p2, err := e.m.CreateCounterPayment(pm.CreateCounterPaymentArgs{
		TenantId: tenant, Id: "p2", ReferenceModule: refModule, ReferenceId: refID, Method: pm.MethodCash,
	})
	if err != nil {
		t.Fatalf("CreateCounterPayment: %v", err)
	}
	if p2.Amount != 20000 {
		t.Errorf("second payment amount = %d, want 20000", p2.Amount)
	}
}
