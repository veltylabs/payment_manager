package tests

import (
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
)

func TestCounter_MissingArgs(t *testing.T) {
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{}}},
	})

	_, err := m.CreateCounterPayment(paymentmanager.CreateCounterPaymentArgs{})
	if err != paymentmanager.ErrMissingArgs {
		t.Errorf("expected ErrMissingArgs, got %v", err)
	}
}

func TestCounter_RetryReturnsStoredWithoutEvent(t *testing.T) {
	db := setupTestDB()
	pub := &mockPublisher{}
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Publisher: pub,
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	args := paymentmanager.CreateCounterPaymentArgs{
		TenantId:        "t1",
		Id:              "p1",
		ReferenceModule: "mod",
		ReferenceId:     "ref1",
		Method:          paymentmanager.MethodCash,
	}

	p1, err := m.CreateCounterPayment(args)
	if err != nil {
		t.Fatal(err)
	}
	eventsCount := len(pub.events)

	p2, err := m.CreateCounterPayment(args)
	if err != nil {
		t.Errorf("unexpected error on retry: %v", err)
	}
	if p1.Id != p2.Id {
		t.Errorf("expected same payment")
	}
	if len(pub.events) != eventsCount {
		t.Errorf("expected no new events on retry")
	}
}

func TestCounter_IdTakenByOtherReference(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	args1 := paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash}
	_, err := m.CreateCounterPayment(args1)
	if err != nil {
		t.Fatal(err)
	}

	args2 := paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref2", Method: paymentmanager.MethodCash}
	_, err = m.CreateCounterPayment(args2)
	if err != paymentmanager.ErrIdTaken {
		t.Errorf("expected ErrIdTaken, got %v", err)
	}
}

func TestCounter_WebpayMethodRejected(t *testing.T) {
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{}}},
	})

	args := paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodWebpay}
	_, err := m.CreateCounterPayment(args)
	if err != paymentmanager.ErrInvalidMethod {
		t.Errorf("expected ErrInvalidMethod, got %v", err)
	}
}

func TestCounter_AlreadyPaid(t *testing.T) {
	db := setupTestDB()
	m, _ := paymentmanager.New(db, paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	args1 := paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash}
	_, err := m.CreateCounterPayment(args1)
	if err != nil {
		t.Fatal(err)
	}

	args2 := paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p2", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash}
	_, err = m.CreateCounterPayment(args2)
	if err != paymentmanager.ErrAlreadyPaid {
		t.Errorf("expected ErrAlreadyPaid, got %v", err)
	}
}

func TestCounter_NotPayable(t *testing.T) {
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 0, currency: paymentmanager.CurrencyCLP, payable: false}}},
	})

	args := paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash}
	_, err := m.CreateCounterPayment(args)
	if err != paymentmanager.ErrNotPayable {
		t.Errorf("expected ErrNotPayable, got %v", err)
	}
}

func TestCounter_CreatesPaidAndPublishes(t *testing.T) {
	pub := &mockPublisher{}
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Publisher: pub,
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{amount: 1000, currency: paymentmanager.CurrencyCLP, payable: true}}},
	})

	args := paymentmanager.CreateCounterPaymentArgs{TenantId: "t1", Id: "p1", ReferenceModule: "mod", ReferenceId: "ref1", Method: paymentmanager.MethodCash}
	p, err := m.CreateCounterPayment(args)
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != paymentmanager.StatusPaid {
		t.Errorf("expected StatusPaid, got %v", p.Status)
	}
	if len(pub.events) != 1 {
		t.Errorf("expected 1 event, got %v", len(pub.events))
	}
}
