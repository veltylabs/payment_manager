package tests

import (
	"testing"

	pm "github.com/veltylabs/payment_manager"
)

func TestComplete_NoGateway(t *testing.T) {
	_, err := newEnvNoGateway(t).m.CompleteOnlinePayment(pm.OnlineReturn{Token: "x"})
	wantErr(t, err, pm.ErrGatewayUnavailable)
}

func TestComplete_EmptyReturn(t *testing.T) {
	_, err := newEnv(t).m.CompleteOnlinePayment(pm.OnlineReturn{})
	wantErr(t, err, pm.ErrMissingArgs)
}

func TestComplete_UnknownAbortToken(t *testing.T) {
	_, err := newEnv(t).m.CompleteOnlinePayment(pm.OnlineReturn{AbortToken: "nope"})
	wantErr(t, err, pm.ErrNotFound)
}

func TestComplete_UnknownBuyOrder(t *testing.T) {
	_, err := newEnv(t).m.CompleteOnlinePayment(pm.OnlineReturn{BuyOrder: "nope"})
	wantErr(t, err, pm.ErrNotFound)
}

func TestComplete_UnknownToken(t *testing.T) {
	_, err := newEnv(t).m.CompleteOnlinePayment(pm.OnlineReturn{Token: "nope"})
	wantErr(t, err, pm.ErrNotFound)
}

func TestComplete_AbortCancels(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	p, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{Token: token, AbortToken: token}) // "volver al sitio" trae ambos
	if err != nil || p.Status != pm.StatusCancelled {
		t.Fatalf("got %s, %v; want CANCELLED", p.Status, err)
	}
}

func TestComplete_TimeoutExpires(t *testing.T) {
	e := newEnv(t)
	e.started(t, "p1", refID)
	p, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{BuyOrder: "p1"})
	if err != nil || p.Status != pm.StatusExpired {
		t.Fatalf("got %s, %v; want EXPIRED", p.Status, err)
	}
}

func TestComplete_AbortOnSettled(t *testing.T) {
	e := newEnv(t)
	token := e.paidOnline(t, "p1", refID)
	p, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{AbortToken: token})
	if err != nil || p.Status != pm.StatusPaid {
		t.Fatalf("got %s, %v; a settled payment is returned as is", p.Status, err)
	}
}

func TestComplete_TimeoutOnSettled(t *testing.T) {
	e := newEnv(t)
	e.paidOnline(t, "p1", refID)
	p, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{BuyOrder: "p1"})
	if err != nil || p.Status != pm.StatusPaid {
		t.Fatalf("got %s, %v", p.Status, err)
	}
}

func TestComplete_RepeatedReturnIsIdempotent(t *testing.T) {
	e := newEnv(t)
	token := e.paidOnline(t, "p1", refID)
	p, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{Token: token})
	if err != nil || p.Status != pm.StatusPaid {
		t.Fatalf("got %s, %v", p.Status, err)
	}
	topics := e.pub.topics()
	if len(topics) != 2 || topics[0] != "money.fact" || topics[1] != pm.EventPaymentPaid {
		t.Fatalf("a repeated return must not publish again: %v", topics)
	}
}

func TestComplete_CommitErrorKeepsPending(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.gw.FailNext(errSimulated)
	_, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{Token: token})
	wantErr(t, err, pm.ErrGatewayFailed)
	if p := e.payment(t, "p1"); p.Status != pm.StatusPending {
		t.Fatalf("status = %s, want PENDING (reconcile closes it)", p.Status)
	}
}

func TestComplete_ApprovedPaysAndPublishes(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.gw.Approve(token)
	p, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{Token: token})
	if err != nil || p.Status != pm.StatusPaid || p.AuthorizationCode == "" || p.CardLast4 == "" || p.PaymentTypeCode == "" || p.PaidAt == 0 || p.Revision != 2 {
		t.Fatalf("payment = %+v, %v", p, err)
	}
	topics := e.pub.topics()
	if len(topics) != 2 || topics[0] != "money.fact" || topics[1] != pm.EventPaymentPaid {
		t.Fatalf("events = %v", topics)
	}
}

func TestComplete_AmountMismatchRefundsAndFails(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.gw.Approve(token)
	e.gw.SetCommitAmount(token, 1000)
	p, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{Token: token})
	wantErr(t, err, pm.ErrAmountMismatch)
	if p.Status != pm.StatusFailed || e.gw.Refunded(token) != 1000 {
		t.Fatalf("status %s, refunded %d; want FAILED and 1000 back", p.Status, e.gw.Refunded(token))
	}
}

func TestComplete_AmountMismatchRefundFailure(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.gw.Approve(token)
	e.gw.SetCommitAmount(token, 1000)
	e.hook.refundErr = errSimulated
	_, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{Token: token})
	wantErr(t, err, pm.ErrGatewayFailed)
	if p := e.payment(t, "p1"); p.Status != pm.StatusPending {
		t.Fatalf("status = %s: with the refund failed the payment stays PENDING for reconcile", p.Status)
	}
}

func TestComplete_RejectedFailsAndPublishes(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.gw.Reject(token)
	p, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{Token: token})
	if err != nil || p.Status != pm.StatusFailed {
		t.Fatalf("got %s, %v", p.Status, err)
	}
	if got := e.pub.topics(); len(got) != 1 || got[0] != pm.EventPaymentFailed {
		t.Fatalf("events = %v", got)
	}
}

func TestComplete_PendingOutcomeKeepsPending(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	p, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{Token: token})
	if err != nil || p.Status != pm.StatusPending {
		t.Fatalf("got %s, %v", p.Status, err)
	}
}

// Otro proceso cierra el pago entre la lectura y la escritura: ErrConflict.
func TestComplete_ConcurrentCloseConflict(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.gw.Approve(token)
	e.hook.onCommit = func() {
		if _, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{AbortToken: token}); err != nil {
			t.Fatalf("concurrent abort: %v", err)
		}
	}
	_, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{Token: token})
	wantErr(t, err, pm.ErrConflict)
}

// Lo que devuelve la pasarela se valida antes de guardarse.
func TestComplete_InvalidGatewayDataNotStored(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.hook.commitResult = &pm.GatewayResult{Outcome: pm.OutcomeAuthorized, Amount: 25000, CardLast4: "1234567890"}
	if _, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{Token: token}); err == nil {
		t.Fatal("a card_last4 over 4 digits must fail validation")
	}
	if p := e.payment(t, "p1"); p.Status != pm.StatusPending || p.CardLast4 != "" {
		t.Fatalf("nothing stored expected, got %+v", p)
	}
}
