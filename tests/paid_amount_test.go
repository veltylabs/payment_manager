package tests

import (
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
)

func TestPaidAmount_Table(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})
	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p2", ReferenceModule: "mod", ReferenceId: "ref2", Method: paymentmanager.MethodCash})
	_, _ = m.RefundPayment(paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p2", Amount: 200, Reason: "test_reason", Method: paymentmanager.MethodCash})

	tests := []struct {
		name      string
		tenant    string
		payment   string
		refMod    string
		refId     string
		err       error
		amount    int64
		currency  string
	}{
		{"pagado", "t1", "p1", "mod", "ref1", nil, 1000, paymentmanager.CurrencyCLP},
		{"parcialmente devuelto", "t1", "p2", "mod", "ref2", nil, 800, paymentmanager.CurrencyCLP},
		{"otro tenant", "t2", "p1", "mod", "ref1", paymentmanager.ErrNotFound, 0, ""},
		{"otra referencia", "t1", "p1", "mod", "ref2", paymentmanager.ErrNotFound, 0, ""},
		{"no existe", "t1", "p3", "mod", "ref1", paymentmanager.ErrNotFound, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amount, currency, err := m.PaidAmount(tt.tenant, tt.payment, tt.refMod, tt.refId)
			if err != tt.err {
				t.Errorf("expected error %v, got %v", tt.err, err)
			}
			if amount != tt.amount || currency != tt.currency {
				t.Errorf("expected %v %v, got %v %v", tt.amount, tt.currency, amount, currency)
			}
		})
	}
}
