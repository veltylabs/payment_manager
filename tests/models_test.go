package tests

import (
	"testing"
	"webtyp.com/model"
	paymentmanager "github.com/veltylabs/payment_manager"
)

func TestModels_RoundTripAndValidate(t *testing.T) {
	args := paymentmanager.CreateCounterPaymentArgs{}
	if err := args.Validate(byte(model.ActionCreate)); err == nil {
		t.Error("expected error for empty args")
	}

	p := paymentmanager.Payment{}
	if err := p.Validate(byte(model.ActionCreate)); err == nil {
		t.Error("expected error for empty payment")
	}
}
