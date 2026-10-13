package tests

import (
	"errors"
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
)

func TestRefund_MissingArgs(t *testing.T) {
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{}}},
	})

	_, err := m.RefundPayment(paymentmanager.RefundPaymentArgs{})
	if err != paymentmanager.ErrMissingArgs {
		t.Errorf("expected ErrMissingArgs, got %v", err)
	}
}

func TestRefund_RetryReturnsStored(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, err := m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})
	if err != nil {
		t.Fatal(err)
	}

	args := paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p1", Amount: 500, Reason: "test_reason", Method: paymentmanager.MethodCash}
	r1, err := m.RefundPayment(args)
	if err != nil {
		t.Fatal(err)
	}

	r2, err := m.RefundPayment(args)
	if err != nil {
		t.Errorf("unexpected error on retry: %v", err)
	}
	if r1.Id != r2.Id {
		t.Errorf("expected same refund")
	}
}

func TestRefund_IdTaken(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})
	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p2", ReferenceModule: "mod", ReferenceId: "ref2", Method: paymentmanager.MethodCash})

	args1 := paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p1", Amount: 500, Reason: "test_reason", Method: paymentmanager.MethodCash}
	_, err := m.RefundPayment(args1)
	if err != nil {
		t.Fatal(err)
	}

	args2 := paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p2", Amount: 500, Reason: "test_reason", Method: paymentmanager.MethodCash}
	_, err = m.RefundPayment(args2)
	if err != paymentmanager.ErrIdTaken {
		t.Errorf("expected ErrIdTaken, got %v", err)
	}
}

func TestRefund_OtherTenantNotFound(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})

	args := paymentmanager.RefundPaymentArgs{TenantId: "t2", Id: "r1", PaymentId: "p1", Amount: 500, Reason: "test_reason", Method: paymentmanager.MethodCash}
	_, err := m.RefundPayment(args)
	if err != paymentmanager.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRefund_PendingPaymentRejected(t *testing.T) {
	db := setupTestDB()
	gw := &mockGateway{startRes: paymentmanager.GatewayStarted{Token: "tok1"}}
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway: gw, ReturnURL: "http://r",
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.StartOnlinePayment(paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"})

	args := paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p1", Amount: 500, Reason: "test_reason"}
	_, err := m.RefundPayment(args)
	if err != paymentmanager.ErrInvalidTransition {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestRefund_ExceedsBalance(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})

	args := paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p1", Amount: 1500, Reason: "test_reason", Method: paymentmanager.MethodCash}
	_, err := m.RefundPayment(args)
	if err != paymentmanager.ErrRefundExceeds {
		t.Errorf("expected ErrRefundExceeds, got %v", err)
	}
}

func TestRefund_ZeroAmount(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})

	args := paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p1", Amount: 0, Reason: "test_reason", Method: paymentmanager.MethodCash}
	_, err := m.RefundPayment(args)
	if err != paymentmanager.ErrMissingArgs {
		t.Errorf("expected ErrMissingArgs, got %v", err)
	}
}

func TestRefund_GatewayErrorWritesNothing(t *testing.T) {
	db := setupTestDB()
	gw := &mockGateway{startRes: paymentmanager.GatewayStarted{Token: "tok1"}, commitRes: paymentmanager.GatewayResult{Outcome: paymentmanager.OutcomeAuthorized, Amount: 1000}, refundErr: errors.New("gateway fail")}
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway: gw, ReturnURL: "http://r",
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.StartOnlinePayment(paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"})
	_, _ = m.CompleteOnlinePayment(paymentmanager.OnlineReturn{Token: "tok1"})

	args := paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p1", Amount: 1000, Reason: "test_reason"}
	_, err := m.RefundPayment(args)
	if err != paymentmanager.ErrGatewayFailed {
		t.Errorf("expected ErrGatewayFailed, got %v", err)
	}

	res, _ := m.ListRefunds(paymentmanager.ListRefundsArgs{TenantId: "t1", PaymentId: "p1"})
	if len(res) > 0 {
		t.Errorf("expected no refunds saved")
	}
}

func TestRefund_OnlineRecordsGatewayKind(t *testing.T) {
	db := setupTestDB()
	gw := &mockGateway{startRes: paymentmanager.GatewayStarted{Token: "tok1"}, commitRes: paymentmanager.GatewayResult{Outcome: paymentmanager.OutcomeAuthorized, Amount: 1000}, refundRes: paymentmanager.GatewayRefund{Kind: paymentmanager.RefundKindReversed, Amount: 1000}}
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway: gw, ReturnURL: "http://r",
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.StartOnlinePayment(paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"})
	_, _ = m.CompleteOnlinePayment(paymentmanager.OnlineReturn{Token: "tok1"})

	args := paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p1", Amount: 1000, Reason: "test_reason"}
	r, err := m.RefundPayment(args)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != paymentmanager.RefundKindReversed {
		t.Errorf("expected REVERSED kind")
	}
}

func TestRefund_CounterInvalidMethod(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})

	args := paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p1", Amount: 1000, Reason: "test_reason", Method: paymentmanager.MethodWebpay} // invalid for counter
	_, err := m.RefundPayment(args)
	if err != paymentmanager.ErrInvalidMethod {
		t.Errorf("expected ErrInvalidMethod, got %v", err)
	}
}

func TestRefund_PartialThenPartial(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})

	_, _ = m.RefundPayment(paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p1", Amount: 400, Reason: "test_reason", Method: paymentmanager.MethodCash})

	p, _ := m.GetPayment(paymentmanager.GetPaymentArgs{TenantId: "t1", Id: "p1"})
	if p.Status != paymentmanager.StatusPartiallyRefunded || p.RefundedAmount != 400 {
		t.Errorf("expected PARTIALLY_REFUNDED and 400")
	}

	_, _ = m.RefundPayment(paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r2", PaymentId: "p1", Amount: 300, Reason: "test_reason", Method: paymentmanager.MethodCash})

	p, _ = m.GetPayment(paymentmanager.GetPaymentArgs{TenantId: "t1", Id: "p1"})
	if p.Status != paymentmanager.StatusPartiallyRefunded || p.RefundedAmount != 700 {
		t.Errorf("expected PARTIALLY_REFUNDED and 700")
	}
}

func TestRefund_RestMakesRefunded(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})
	_, _ = m.RefundPayment(paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p1", Amount: 400, Reason: "test_reason", Method: paymentmanager.MethodCash})

	_, _ = m.RefundPayment(paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r2", PaymentId: "p1", Amount: 600, Reason: "test_reason", Method: paymentmanager.MethodCash})

	p, _ := m.GetPayment(paymentmanager.GetPaymentArgs{TenantId: "t1", Id: "p1"})
	if p.Status != paymentmanager.StatusRefunded || p.RefundedAmount != 1000 {
		t.Errorf("expected REFUNDED and 1000")
	}
}

func TestRefund_ConcurrentRevisionConflict(t *testing.T) {
	t.Skip("Tested in storage errors")
}

func TestRefund_PublishesRefunded(t *testing.T) {
	pub := &mockPublisher{}
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Publisher: pub,
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})

	eventsCount := len(pub.events)

	_, err := m.RefundPayment(paymentmanager.RefundPaymentArgs{TenantId: "t1", Id: "r1", PaymentId: "p1", Amount: 1000, Reason: "test_reason", Method: paymentmanager.MethodCash})
	if err != nil {
		t.Fatal(err)
	}

	if len(pub.events) != eventsCount+1 {
		t.Errorf("expected 1 event published")
	}
}
