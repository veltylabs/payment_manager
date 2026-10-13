package tests

import (
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
	"webtyp.com/time"
)

func TestGetPayment(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})

	p, err := m.GetPayment(paymentmanager.GetPaymentArgs{TenantId: "t1", Id: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Id != "p1" {
		t.Errorf("expected p1")
	}
}

func TestListPaymentsByReference(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})

	res, err := m.ListPaymentsByReference(paymentmanager.ListPaymentsByReferenceArgs{TenantId: "t1", ReferenceModule: "mod", ReferenceId: "ref1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Id != "p1" {
		t.Errorf("expected p1")
	}
}

func TestListPayments_DateRange(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})

	now := time.Now() / 1_000_000_000
	res, err := m.ListPayments(paymentmanager.ListPaymentsArgs{TenantId: "t1", From: int64(now) - 100, To: int64(now) + 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Id != "p1" {
		t.Errorf("expected p1")
	}

	res2, _ := m.ListPayments(paymentmanager.ListPaymentsArgs{TenantId: "t1", From: int64(now) + 100, To: int64(now) + 200})
	if len(res2) != 0 {
		t.Errorf("expected empty")
	}
}

func TestListRefunds(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})
	_, _ = m.RefundPayment(paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p1", Amount: 200, Reason: "test_reason", Method: paymentmanager.MethodCash})

	res, err := m.ListRefunds(paymentmanager.ListRefundsArgs{TenantId: "t1", PaymentId: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Id != "r1" {
		t.Errorf("expected r1")
	}
}
