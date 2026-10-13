package tests

import (
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
	"webtyp.com/router/mock"
)

func TestConformance_MountOperations(t *testing.T) {
	m, _ := paymentmanager.New(setupTestDB(), paymentmanager.Deps{
		Charges: []paymentmanager.ChargeSource{{Module: "mod", Reader: mockChargeReader{}}},
	})

	reg := &mock.Router{}
	m.MountOperations(reg)

	if m.ModelName() != paymentmanager.ModelName {
		t.Errorf("expected %q, got %q", paymentmanager.ModelName, m.ModelName())
	}
}
