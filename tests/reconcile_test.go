package tests

import (
	"errors"
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
	"webtyp.com/time"
)

func TestReconcile_RecentPendingUntouched(t *testing.T) {
	db := setupTestDB()
	gw := &mockGateway{startRes: paymentmanager.GatewayStarted{Token: "tok1"}}
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway: gw, ReturnURL: "http://r",
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.StartOnlinePayment(paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"})

	now := time.Now() / 1_000_000_000
	res, err := m.ReconcileOnlinePayments("t1", int64(now))
	if err != nil {
		t.Fatal(err)
	}
	if res.Closed != 0 {
		t.Errorf("expected 0 closed, got %v", res.Closed)
	}
}

func TestReconcile_OlderThanWindowExpiresWithoutGateway(t *testing.T) {
	db := setupTestDB()
	gw := &mockGateway{startRes: paymentmanager.GatewayStarted{Token: "tok1"}, statusErr: errors.New("gateway fail")}
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway: gw, ReturnURL: "http://r",
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.StartOnlinePayment(paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"})

	now := time.Now() / 1_000_000_000 + paymentmanager.GatewayStatusWindow + 1
	res, err := m.ReconcileOnlinePayments("t1", int64(now))
	if err != nil {
		t.Fatal(err)
	}
	if res.Closed != 1 {
		t.Errorf("expected 1 closed, got %v", res.Closed)
	}

	p, _ := m.GetPayment(paymentmanager.GetPaymentArgs{TenantId: "t1", Id: "p1"})
	if p.Status != paymentmanager.StatusExpired {
		t.Errorf("expected EXPIRED")
	}
}

func TestReconcile_GatewayErrorSkipsAndContinues(t *testing.T) {
	db := setupTestDB()
	gw := &mockGateway{startRes: paymentmanager.GatewayStarted{Token: "tok1"}, statusErr: errors.New("gateway fail")}
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway: gw, ReturnURL: "http://r",
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.StartOnlinePayment(paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"})

	now := time.Now() / 1_000_000_000 + paymentmanager.OnlinePendingTimeout + 1
	res, err := m.ReconcileOnlinePayments("t1", int64(now))
	if err != nil {
		t.Fatal(err)
	}
	if res.Closed != 0 {
		t.Errorf("expected 0 closed, got %v", res.Closed)
	}
}

func TestReconcile_AuthorizedPays(t *testing.T) {
	db := setupTestDB()
	gw := &mockGateway{startRes: paymentmanager.GatewayStarted{Token: "tok1"}, statusRes: paymentmanager.GatewayResult{Outcome: paymentmanager.OutcomeAuthorized, Amount: 1000}}
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway: gw, ReturnURL: "http://r",
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.StartOnlinePayment(paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"})

	now := time.Now() / 1_000_000_000 + paymentmanager.OnlinePendingTimeout + 1
	res, err := m.ReconcileOnlinePayments("t1", int64(now))
	if err != nil {
		t.Fatal(err)
	}
	if res.Closed != 1 {
		t.Errorf("expected 1 closed")
	}

	p, _ := m.GetPayment(paymentmanager.GetPaymentArgs{TenantId: "t1", Id: "p1"})
	if p.Status != paymentmanager.StatusPaid {
		t.Errorf("expected PAID")
	}
}

func TestReconcile_RejectedFails(t *testing.T) {
	db := setupTestDB()
	gw := &mockGateway{startRes: paymentmanager.GatewayStarted{Token: "tok1"}, statusRes: paymentmanager.GatewayResult{Outcome: paymentmanager.OutcomeRejected}}
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway: gw, ReturnURL: "http://r",
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.StartOnlinePayment(paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"})

	now := time.Now() / 1_000_000_000 + paymentmanager.OnlinePendingTimeout + 1
	res, err := m.ReconcileOnlinePayments("t1", int64(now))
	if err != nil {
		t.Fatal(err)
	}
	if res.Closed != 1 {
		t.Errorf("expected 1 closed")
	}

	p, _ := m.GetPayment(paymentmanager.GetPaymentArgs{TenantId: "t1", Id: "p1"})
	if p.Status != paymentmanager.StatusFailed {
		t.Errorf("expected FAILED")
	}
}

func TestReconcile_StillPendingExpires(t *testing.T) {
	db := setupTestDB()
	gw := &mockGateway{startRes: paymentmanager.GatewayStarted{Token: "tok1"}, statusRes: paymentmanager.GatewayResult{Outcome: paymentmanager.OutcomePending}}
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway: gw, ReturnURL: "http://r",
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, _ = m.StartOnlinePayment(paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"})

	now := time.Now() / 1_000_000_000 + paymentmanager.OnlinePendingTimeout + 1
	res, err := m.ReconcileOnlinePayments("t1", int64(now))
	if err != nil {
		t.Fatal(err)
	}
	if res.Closed != 1 {
		t.Errorf("expected 1 closed")
	}

	p, _ := m.GetPayment(paymentmanager.GetPaymentArgs{TenantId: "t1", Id: "p1"})
	if p.Status != paymentmanager.StatusExpired {
		t.Errorf("expected EXPIRED")
	}
}
