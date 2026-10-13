package tests

import (
	"testing"

	pm "github.com/veltylabs/payment_manager"
)

func TestTransition_Table(t *testing.T) {
	cases := []struct{ from, event, to string }{
		{pm.StatusPending, pm.EventPay, pm.StatusPaid},
		{pm.StatusPending, pm.EventFail, pm.StatusFailed},
		{pm.StatusPending, pm.EventCancel, pm.StatusCancelled},
		{pm.StatusPending, pm.EventExpire, pm.StatusExpired},
		{pm.StatusPaid, pm.EventRefundPartial, pm.StatusPartiallyRefunded},
		{pm.StatusPaid, pm.EventRefundTotal, pm.StatusRefunded},
		{pm.StatusPartiallyRefunded, pm.EventRefundPartial, pm.StatusPartiallyRefunded},
		{pm.StatusPartiallyRefunded, pm.EventRefundTotal, pm.StatusRefunded},
	}
	for _, c := range cases {
		got, err := pm.Transition(c.from, c.event)
		if err != nil || got != c.to {
			t.Errorf("Transition(%s, %s) = %q, %v; want %q", c.from, c.event, got, err, c.to)
		}
	}
}

func TestTransition_Invalid(t *testing.T) {
	_, err := pm.Transition(pm.StatusRefunded, pm.EventPay)
	wantErr(t, err, pm.ErrInvalidTransition)
}
