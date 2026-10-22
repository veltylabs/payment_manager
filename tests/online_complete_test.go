package tests

import (
	"testing"

	pm "github.com/veltylabs/payment_manager"
)

func TestComplete_G1_AuthorizedSameAmount(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.gw.Approve(token)
	r, _ := e.gw.Result(token)

	p, err := e.m.CompleteOnlinePayment(r)
	if err != nil || p.Status != pm.StatusPaid || p.AuthorizationCode == "" || p.CardLast4 == "" || p.PaymentTypeCode == "" || p.PaidAt == 0 || p.Revision != 2 {
		t.Fatalf("payment = %+v, %v", p, err)
	}
	topics := e.pub.topics()
	if len(topics) != 2 || topics[0] != "money.fact" || topics[1] != pm.EventPaymentPaid {
		t.Fatalf("events = %v", topics)
	}
	if p.Provider != "sandbox" || p.Method != pm.MethodCard {
		t.Fatalf("got provider %s, method %s; want sandbox, card", p.Provider, p.Method)
	}
}

func TestComplete_G2_Rejected(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.gw.Reject(token)
	r, _ := e.gw.Result(token)

	p, err := e.m.CompleteOnlinePayment(r)
	if err != nil || p.Status != pm.StatusFailed {
		t.Fatalf("got %s, %v", p.Status, err)
	}
	if got := e.pub.topics(); len(got) != 1 || got[0] != pm.EventPaymentFailed {
		t.Fatalf("events = %v", got)
	}
}

func TestComplete_G3_Cancelled(t *testing.T) {
	e := newEnv(t)
	e.started(t, "p1", refID)
	r := pm.OnlineResult{PaymentId: "p1", Result: pm.GatewayResult{Outcome: pm.OutcomeCancelled}}

	p, err := e.m.CompleteOnlinePayment(r)
	if err != nil || p.Status != pm.StatusCancelled {
		t.Fatalf("got %s, %v; want CANCELLED", p.Status, err)
	}
}

func TestComplete_G4_Expired(t *testing.T) {
	e := newEnv(t)
	e.started(t, "p1", refID)
	r := pm.OnlineResult{PaymentId: "p1", Result: pm.GatewayResult{Outcome: pm.OutcomeExpired}}

	p, err := e.m.CompleteOnlinePayment(r)
	if err != nil || p.Status != pm.StatusExpired {
		t.Fatalf("got %s, %v; want EXPIRED", p.Status, err)
	}
}

func TestComplete_G5_PendingStaysPending(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	r, _ := e.gw.Result(token)

	p, err := e.m.CompleteOnlinePayment(r)
	if err != nil || p.Status != pm.StatusPending {
		t.Fatalf("got %s, %v", p.Status, err)
	}
}

func TestComplete_G6_AuthorizedOtherAmount(t *testing.T) {
	e := newEnv(t)
	token := e.started(t, "p1", refID)
	e.gw.Approve(token)
	e.gw.SetCommitAmount(token, 1000)
	r, _ := e.gw.Result(token)

	p, err := e.m.CompleteOnlinePayment(r)
	wantErr(t, err, pm.ErrAmountMismatch)
	if p.Status != pm.StatusFailed || e.gw.Refunded(token) != 1000 {
		t.Fatalf("status %s, refunded %d; want FAILED and 1000 back", p.Status, e.gw.Refunded(token))
	}
}

func TestComplete_G7_PaymentAlreadyPaid(t *testing.T) {
	e := newEnv(t)
	token := e.paidOnline(t, "p1", refID)
	r, _ := e.gw.Result(token)

	p, err := e.m.CompleteOnlinePayment(r)
	if err != nil || p.Status != pm.StatusPaid {
		t.Fatalf("got %s, %v", p.Status, err)
	}
	topics := e.pub.topics()
	if len(topics) != 2 || topics[0] != "money.fact" || topics[1] != pm.EventPaymentPaid {
		t.Fatalf("a repeated return must not publish again: %v", topics)
	}
}

func TestComplete_G8_UnknownPaymentId(t *testing.T) {
	e := newEnv(t)
	_, err := e.m.CompleteOnlinePayment(pm.OnlineResult{PaymentId: "nope", Result: pm.GatewayResult{Outcome: pm.OutcomeAuthorized}})
	wantErr(t, err, pm.ErrNotFound)
}

func TestComplete_G9_EmptyPaymentId(t *testing.T) {
	e := newEnv(t)
	_, err := e.m.CompleteOnlinePayment(pm.OnlineResult{})
	wantErr(t, err, pm.ErrMissingArgs)
}

func TestComplete_G10_NoGateway(t *testing.T) {
	e := newEnvNoGateway(t)
	_, err := e.m.CompleteOnlinePayment(pm.OnlineResult{PaymentId: "x"})
	wantErr(t, err, pm.ErrGatewayUnavailable)
}
