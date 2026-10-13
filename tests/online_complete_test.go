package tests

import (
	"errors"
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
)

func TestComplete_EmptyReturn(t *testing.T) {
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{}}},
	})
	_, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{})
	if err != paymentmanager.ErrMissingArgs {
		t.Errorf("expected ErrMissingArgs, got %v", err)
	}
}

func TestComplete_UnknownAbortToken(t *testing.T) {
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{}}},
	})
	_, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{AbortToken: "unknown"})
	if err != paymentmanager.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestComplete_UnknownBuyOrder(t *testing.T) {
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{}}},
	})
	_, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{BuyOrder: "unknown"})
	if err != paymentmanager.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestComplete_UnknownToken(t *testing.T) {
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{}}},
	})
	_, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{Token: "unknown"})
	if err != paymentmanager.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func setupCompleteTest(t *testing.T) (*paymentmanager.Module, *mockGateway, *mockPublisher) {
	db := setupTestDB()
	gw := &mockGateway{startRes: paymentmanager.GatewayStarted{Token: "tok123"}}
	pub := &mockPublisher{}
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway:   gw,
		ReturnURL: "http://return",
		Publisher: pub,
		Charges:   []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, err := m.StartOnlinePayment(paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	return m, gw, pub
}

func TestComplete_AbortOnSettled(t *testing.T) {
	m, gw, _ := setupCompleteTest(t)
	gw.commitRes = paymentmanager.GatewayResult{Outcome: paymentmanager.OutcomeAuthorized, Amount: 1000}
	_, _ = m.CompleteOnlinePayment(paymentmanager.OnlineReturn{Token: "tok123"}) // settle it

	p, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{AbortToken: "tok123"})
	if err != nil {
		t.Errorf("expected no error (idempotent), got %v", err)
	}
	if p.Status != paymentmanager.StatusPaid {
		t.Errorf("expected it to stay PAID")
	}
}

func TestComplete_TimeoutOnSettled(t *testing.T) {
	m, gw, _ := setupCompleteTest(t)
	gw.commitRes = paymentmanager.GatewayResult{Outcome: paymentmanager.OutcomeAuthorized, Amount: 1000}
	_, _ = m.CompleteOnlinePayment(paymentmanager.OnlineReturn{Token: "tok123"}) // settle it

	p, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{BuyOrder: "p1"})
	if err != nil {
		t.Errorf("expected no error (idempotent), got %v", err)
	}
	if p.Status != paymentmanager.StatusPaid {
		t.Errorf("expected it to stay PAID")
	}
}

func TestComplete_RepeatedReturnIsIdempotent(t *testing.T) {
	m, gw, _ := setupCompleteTest(t)
	gw.commitRes = paymentmanager.GatewayResult{Outcome: paymentmanager.OutcomeAuthorized, Amount: 1000}
	_, _ = m.CompleteOnlinePayment(paymentmanager.OnlineReturn{Token: "tok123"}) // settle it

	p, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{Token: "tok123"})
	if err != nil {
		t.Errorf("expected no error (idempotent), got %v", err)
	}
	if p.Status != paymentmanager.StatusPaid {
		t.Errorf("expected it to stay PAID")
	}
}

func TestComplete_AbortCancels(t *testing.T) {
	m, _, _ := setupCompleteTest(t)
	p, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{AbortToken: "tok123"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != paymentmanager.StatusCancelled {
		t.Errorf("expected CANCELLED, got %v", p.Status)
	}
}

func TestComplete_TimeoutExpires(t *testing.T) {
	m, _, _ := setupCompleteTest(t)
	p, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{BuyOrder: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != paymentmanager.StatusExpired {
		t.Errorf("expected EXPIRED, got %v", p.Status)
	}
}

func TestComplete_CommitErrorKeepsPending(t *testing.T) {
	m, gw, _ := setupCompleteTest(t)
	gw.commitErr = errors.New("gateway network error")

	_, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{Token: "tok123"})
	if err != paymentmanager.ErrGatewayFailed {
		t.Errorf("expected ErrGatewayFailed, got %v", err)
	}

	p, _ := m.GetPayment(paymentmanager.GetPaymentArgs{TenantId: "t1", Id: "p1"})
	if p.Status != paymentmanager.StatusPending {
		t.Errorf("expected to stay PENDING, got %v", p.Status)
	}
}

func TestComplete_ApprovedPaysAndPublishes(t *testing.T) {
	m, gw, pub := setupCompleteTest(t)
	gw.commitRes = paymentmanager.GatewayResult{
		Outcome:           paymentmanager.OutcomeAuthorized,
		Amount:            1000,
		AuthorizationCode: "1234",
		PaymentTypeCode:   "VN",
		Installments:      1,
		CardLast4:         "6623",
	}

	p, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{Token: "tok123"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != paymentmanager.StatusPaid {
		t.Errorf("expected PAID, got %v", p.Status)
	}
	if p.AuthorizationCode != "1234" || p.PaymentTypeCode != "VN" || p.Installments != 1 || p.CardLast4 != "6623" {
		t.Errorf("missing gateway data on payment")
	}
	if p.PaidAt == 0 {
		t.Errorf("expected PaidAt to be set")
	}
	if len(pub.events) != 1 {
		t.Errorf("expected event published")
	}
}

func TestComplete_AmountMismatchRefundsAndFails(t *testing.T) {
	m, gw, _ := setupCompleteTest(t)
	gw.commitRes = paymentmanager.GatewayResult{Outcome: paymentmanager.OutcomeAuthorized, Amount: 500} // mismatched amount

	p, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{Token: "tok123"})
	if err != paymentmanager.ErrAmountMismatch {
		t.Errorf("expected ErrAmountMismatch, got %v", err)
	}
	if p.Status != paymentmanager.StatusFailed {
		t.Errorf("expected FAILED, got %v", p.Status)
	}
	if gw.refundAmount != 500 {
		t.Errorf("expected gateway refund to be called with 500, got %v", gw.refundAmount)
	}
}

func TestComplete_RejectedFailsAndPublishes(t *testing.T) {
	m, gw, pub := setupCompleteTest(t)
	gw.commitRes = paymentmanager.GatewayResult{Outcome: paymentmanager.OutcomeRejected}

	p, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{Token: "tok123"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != paymentmanager.StatusFailed {
		t.Errorf("expected FAILED, got %v", p.Status)
	}
	if len(pub.events) != 1 {
		t.Errorf("expected event published")
	}
}

func TestComplete_PendingOutcomeKeepsPending(t *testing.T) {
	m, gw, _ := setupCompleteTest(t)
	gw.commitRes = paymentmanager.GatewayResult{Outcome: paymentmanager.OutcomePending}

	p, err := m.CompleteOnlinePayment(paymentmanager.OnlineReturn{Token: "tok123"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != paymentmanager.StatusPending {
		t.Errorf("expected PENDING, got %v", p.Status)
	}
}
