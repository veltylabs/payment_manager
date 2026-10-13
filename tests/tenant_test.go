package tests

import (
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
)

func TestTenant_ReadsRefundsAndReconcileAreScoped(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})
	_, _ = m.RefundPayment(paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p1", Amount: 200, Reason: "test_reason", Method: paymentmanager.MethodCash})

	// GetPayment
	_, err := m.GetPayment(paymentmanager.GetPaymentArgs{TenantId: "t2", Id: "p1"})
	if err != paymentmanager.ErrNotFound {
		t.Errorf("expected ErrNotFound for t2, got %v", err)
	}

	// ListPaymentsByReference
	listRef, _ := m.ListPaymentsByReference(paymentmanager.ListPaymentsByReferenceArgs{TenantId: "t2", ReferenceModule: "mod", ReferenceId: "ref1"})
	if len(listRef) != 0 {
		t.Errorf("expected empty list for t2")
	}

	// ListPayments
	list, _ := m.ListPayments(paymentmanager.ListPaymentsArgs{TenantId: "t2", From: 0, To: 9999999999})
	if len(list) != 0 {
		t.Errorf("expected empty list for t2")
	}

	// ListRefunds
	listRefunds, _ := m.ListRefunds(paymentmanager.ListRefundsArgs{TenantId: "t2", PaymentId: "p1"})
	if len(listRefunds) != 0 {
		t.Errorf("expected empty list for t2")
	}
}
