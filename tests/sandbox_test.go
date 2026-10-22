package tests

import (
	"testing"

	pm "github.com/veltylabs/payment_manager"
	"github.com/veltylabs/payment_manager/sandbox"
)

func TestSandbox_Start(t *testing.T) {
	g := sandbox.New("https://pay.example")
	s, err := g.Start(pm.GatewayStart{PaymentId: "b1", Amount: 100})
	if err != nil || s.Token != sandbox.TokenPrefix+"b1" || s.RedirectURL != "https://pay.example" {
		t.Fatalf("Start = %+v, %v", s, err)
	}
	r, err := g.Status(s.Token)
	if err != nil || r.Outcome != pm.OutcomePending || r.Amount != 100 || r.AuthorizationCode != "" {
		t.Fatalf("a new transaction is pending: %+v, %v", r, err)
	}
}

func TestSandbox_ApproveAndReject(t *testing.T) {
	g := sandbox.New("/")
	s, _ := g.Start(pm.GatewayStart{PaymentId: "b1", Amount: 100})
	g.Approve(s.Token)
	if res, _ := g.Result(s.Token); res.Result.Outcome != pm.OutcomeAuthorized || res.Result.AuthorizationCode != sandbox.AuthorizationCode || res.Result.CardLast4 == "" {
		t.Fatalf("approved = %+v", res)
	}
	g.Reject(s.Token)
	if res, _ := g.Result(s.Token); res.Result.Outcome != pm.OutcomeRejected {
		t.Fatalf("rejected = %+v", res)
	}
	g.Approve("unknown") // a no-op, not a panic
}

func TestSandbox_FailNext(t *testing.T) {
	g := sandbox.New("/")
	g.FailNext(errSimulated)
	if _, err := g.Start(pm.GatewayStart{PaymentId: "b1"}); err != errSimulated {
		t.Fatalf("Start err = %v", err)
	}
	s, err := g.Start(pm.GatewayStart{PaymentId: "b1", Amount: 10})
	if err != nil {
		t.Fatalf("the failure applies to one call only: %v", err)
	}
	g.FailNext(errSimulated)
	if _, err := g.Result(s.Token); err != errSimulated {
		t.Fatalf("Commit err = %v", err)
	}
	g.FailNext(errSimulated)
	if _, err := g.Refund(s.Token, 1); err != errSimulated {
		t.Fatalf("Refund err = %v", err)
	}
}

func TestSandbox_SetCommitAmount(t *testing.T) {
	g := sandbox.New("/")
	s, _ := g.Start(pm.GatewayStart{PaymentId: "b1", Amount: 100})
	g.SetCommitAmount(s.Token, 7)
	if res, _ := g.Result(s.Token); res.Result.Amount != 7 {
		t.Fatalf("amount = %d", res.Result.Amount)
	}
}

func TestSandbox_RefundTotalThenPartial(t *testing.T) {
	g := sandbox.New("/")
	s, _ := g.Start(pm.GatewayStart{PaymentId: "b1", Amount: 100})
	if r, err := g.Refund(s.Token, 100); err != nil || r.Kind != pm.RefundKindReversed || r.Amount != 100 {
		t.Fatalf("first total refund = %+v, %v", r, err)
	}
	s2, _ := g.Start(pm.GatewayStart{PaymentId: "b2", Amount: 100})
	if r, _ := g.Refund(s2.Token, 40); r.Kind != pm.RefundKindNullified {
		t.Fatalf("partial = %+v", r)
	}
	if r, _ := g.Refund(s2.Token, 60); r.Kind != pm.RefundKindNullified {
		t.Fatalf("second refund is a nullification = %+v", r)
	}
	if g.Refunded(s2.Token) != 100 || g.Refunded("unknown") != 0 {
		t.Fatalf("refunded = %d", g.Refunded(s2.Token))
	}
}

func TestSandbox_UnknownToken(t *testing.T) {
	g := sandbox.New("/")
	if _, err := g.Result("nope"); err != sandbox.ErrUnknownToken {
		t.Fatalf("Commit err = %v", err)
	}
	if _, err := g.Refund("nope", 1); err != sandbox.ErrUnknownToken {
		t.Fatalf("Refund err = %v", err)
	}
}
