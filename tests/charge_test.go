package tests

import (
	"errors"
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
)

func TestCharge_UnknownModule(t *testing.T) {
	m, err := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{
			{Module: "other_module", Reader: mockChargeReader{}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, _, err = m.Charge("tenant1", "unknown_module", "ref1")
	if err != paymentmanager.ErrUnknownReference {
		t.Errorf("expected ErrUnknownReference, got %v", err)
	}
}

func TestCharge_ReaderError(t *testing.T) {
	expectedErr := errors.New("reader error")
	m, err := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{
			{Module: "my_module", Reader: mockChargeReader{err: expectedErr}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, _, err = m.Charge("tenant1", "my_module", "ref1")
	if err != expectedErr {
		t.Errorf("expected reader error, got %v", err)
	}
}

func TestCharge_UnsupportedCurrency(t *testing.T) {
	m, err := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{
			{Module: "my_module", Reader: mockChargeReader{amount: 100, currency: "EUR", payable: true}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, _, err = m.Charge("tenant1", "my_module", "ref1")
	if err != paymentmanager.ErrUnsupportedCurrency {
		t.Errorf("expected ErrUnsupportedCurrency, got %v", err)
	}
}

func TestCharge_OK(t *testing.T) {
	m, err := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{
			{Module: "my_module", Reader: mockChargeReader{amount: 25000, currency: paymentmanager.CurrencyCLP, payable: true}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	amount, currency, payable, err := m.Charge("tenant1", "my_module", "ref1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if amount != 25000 || currency != paymentmanager.CurrencyCLP || payable != true {
		t.Errorf("expected 25000 CLP true, got %v %v %v", amount, currency, payable)
	}
}
