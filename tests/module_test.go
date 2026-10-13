package tests

import (
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
)

func TestNew_RequiresCharges(t *testing.T) {
	_, err := paymentmanager.New(setupTestDB(), paymentmanager.Deps{})
	if err == nil || err.Error() != "payment_manager: Deps.Charges is required" {
		t.Errorf("expected error, got %v", err)
	}
}

func TestNew_GatewayRequiresReturnURL(t *testing.T) {
	_, err := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Gateway: &mockGateway{},
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{}}},
	})
	if err == nil || err.Error() != "payment_manager: Deps.ReturnURL is required with a Gateway" {
		t.Errorf("expected error, got %v", err)
	}
}
