package tests

import (
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
)

func TestTransition_Table(t *testing.T) {
	tests := []struct {
		current  string
		event    string
		expected string
	}{
		{paymentmanager.StatusPending, paymentmanager.EventPay, paymentmanager.StatusPaid},
		{paymentmanager.StatusPending, paymentmanager.EventFail, paymentmanager.StatusFailed},
		{paymentmanager.StatusPending, paymentmanager.EventCancel, paymentmanager.StatusCancelled},
		{paymentmanager.StatusPending, paymentmanager.EventExpire, paymentmanager.StatusExpired},
		{paymentmanager.StatusPaid, paymentmanager.EventRefundPartial, paymentmanager.StatusPartiallyRefunded},
		{paymentmanager.StatusPaid, paymentmanager.EventRefundTotal, paymentmanager.StatusRefunded},
		{paymentmanager.StatusPartiallyRefunded, paymentmanager.EventRefundPartial, paymentmanager.StatusPartiallyRefunded},
		{paymentmanager.StatusPartiallyRefunded, paymentmanager.EventRefundTotal, paymentmanager.StatusRefunded},
	}

	for _, tt := range tests {
		t.Run(tt.current+"_"+tt.event, func(t *testing.T) {
			got, err := paymentmanager.Transition(tt.current, tt.event)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("Transition(%q, %q) = %q, want %q", tt.current, tt.event, got, tt.expected)
			}
		})
	}
}

func TestTransition_Invalid(t *testing.T) {
	_, err := paymentmanager.Transition(paymentmanager.StatusPaid, paymentmanager.EventPay)
	if err != paymentmanager.ErrInvalidTransition {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}
