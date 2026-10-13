package tests

import (
	"errors"
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
)

type mockGateway struct {
	startErr  error
	commitErr error
	statusErr error
	refundErr error

	startReq     paymentmanager.GatewayStart
	startRes     paymentmanager.GatewayStarted
	commitRes    paymentmanager.GatewayResult
	statusRes    paymentmanager.GatewayResult
	refundRes    paymentmanager.GatewayRefund
	refundAmount int64
}

func (m *mockGateway) Start(req paymentmanager.GatewayStart) (paymentmanager.GatewayStarted, error) {
	m.startReq = req
	return m.startRes, m.startErr
}

func (m *mockGateway) Commit(token string) (paymentmanager.GatewayResult, error) {
	return m.commitRes, m.commitErr
}

func (m *mockGateway) Status(token string) (paymentmanager.GatewayResult, error) {
	return m.statusRes, m.statusErr
}

func (m *mockGateway) Refund(token string, amount int64) (paymentmanager.GatewayRefund, error) {
	m.refundAmount = amount
	return m.refundRes, m.refundErr
}

func TestStartOnline_NoGateway(t *testing.T) {
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	args := paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"}
	_, err := m.StartOnlinePayment(args)
	if err != paymentmanager.ErrGatewayUnavailable {
		t.Errorf("expected ErrGatewayUnavailable, got %v", err)
	}
}

func TestStartOnline_MissingArgs(t *testing.T) {
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Gateway:   &mockGateway{},
		ReturnURL: "http://return",
		Charges:   []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, err := m.StartOnlinePayment(paymentmanager.StartOnlinePaymentArgs{})
	if err != paymentmanager.ErrMissingArgs {
		t.Errorf("expected ErrMissingArgs, got %v", err)
	}
}

func TestStartOnline_IdTaken(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway:   &mockGateway{},
		ReturnURL: "http://return",
		Charges:   []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	args := paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"}
	_, err := m.StartOnlinePayment(args)
	if err != nil {
		t.Fatal(err)
	}

	_, err = m.StartOnlinePayment(args)
	if err != paymentmanager.ErrIdTaken {
		t.Errorf("expected ErrIdTaken, got %v", err)
	}
}

func TestStartOnline_AlreadyPaid(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway:   &mockGateway{},
		ReturnURL: "http://return",
		Charges:   []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	_, err := m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash})
	if err != nil {
		t.Fatal(err)
	}

	args := paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p2", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"}
	_, err = m.StartOnlinePayment(args)
	if err != paymentmanager.ErrAlreadyPaid {
		t.Errorf("expected ErrAlreadyPaid, got %v", err)
	}
}

func TestStartOnline_NotPayable(t *testing.T) {
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Gateway:   &mockGateway{},
		ReturnURL: "http://return",
		Charges:   []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 0, currency: paymentmanager.CurrencyCLP, payable: false}}},
	})

	args := paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"}
	_, err := m.StartOnlinePayment(args)
	if err != paymentmanager.ErrNotPayable {
		t.Errorf("expected ErrNotPayable, got %v", err)
	}
}

func TestStartOnline_GatewayErrorMarksFailed(t *testing.T) {
	db := setupTestDB()
	gw := &mockGateway{startErr: errors.New("gateway down")}
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway:   gw,
		ReturnURL: "http://return",
		Charges:   []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	args := paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"}
	_, err := m.StartOnlinePayment(args)
	if err != paymentmanager.ErrGatewayFailed {
		t.Errorf("expected ErrGatewayFailed, got %v", err)
	}

	p, _ := m.GetPayment(paymentmanager.GetPaymentArgs{TenantId: "t1", Id: "p1"})
	if p.Status != paymentmanager.StatusFailed {
		t.Errorf("expected payment to be marked FAILED, got %v", p.Status)
	}
}

func TestStartOnline_PendingWithToken(t *testing.T) {
	db := setupTestDB()
	gw := &mockGateway{startRes: paymentmanager.GatewayStarted{Token: "token123", RedirectURL: "http://gw"}}
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Gateway:   gw,
		ReturnURL: "http://return",
		Charges:   []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	args := paymentmanager.StartOnlinePaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", ReturnTo: "hash"}
	res, err := m.StartOnlinePayment(args)
	if err != nil {
		t.Fatal(err)
	}

	if res.Token != "token123" || res.RedirectUrl != "http://gw" || res.PaymentId != "p1" {
		t.Errorf("unexpected result: %+v", res)
	}

	if gw.startReq.BuyOrder != "p1" || gw.startReq.Amount != 1000 || gw.startReq.ReturnURL != "http://return" {
		t.Errorf("unexpected gateway request: %+v", gw.startReq)
	}

	p, _ := m.GetPayment(paymentmanager.GetPaymentArgs{TenantId: "t1", Id: "p1"})
	if p.Status != paymentmanager.StatusPending {
		t.Errorf("expected pending payment, got %v", p.Status)
	}
}
