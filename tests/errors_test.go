package tests

import (
	"testing"
	pm "github.com/veltylabs/payment_manager"
)

func TestErrors_KindsToStatus(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", "r1")
	r := opRouter(e)

	cases := []struct {
		op string
		body string
		want int
	}{
		// model.Invalid -> 400
		{pm.OpGetCharge, `{}`, 400},

		// model.Conflict -> 409
		{pm.OpCreateCounterPayment, `{"id":"p1", "tenant_id":"t1", "reference_module":"m", "reference_id":"r", "method":"cash"}`, 409},

		// model.NotFound -> 404
		{pm.OpGetPaymentStatus, `{"id":"not-found", "tenant_id":"t1"}`, 404},
	}

	for _, c := range cases {
		status, _ := call(r, c.op, c.body)
		if status != c.want {
			t.Errorf("call %s = %d, want %d", c.op, status, c.want)
		}
	}
}



func TestErrors_PlainErrorTo500(t *testing.T) {
	e := newEnv(t)
	e.conn.failOn("payment", 1, 0) // action create is 1
	r := opRouter(e)

	status, text := call(r, pm.OpStartOnlinePayment, `{"id":"id", "tenant_id":"t1", "reference_module":"m", "reference_id":"r"}`)
	if status != 500 {
		t.Errorf("got %d, want 500 for plain error", status)
	}
	if text == errSimulated.Error() {
		t.Errorf("plain error text was leaked: %s", text)
	}
}
