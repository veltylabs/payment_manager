package tests

import (
	"testing"

	pm "github.com/veltylabs/payment_manager"
)

func startArgs(id string) pm.StartOnlinePaymentArgs {
	return pm.StartOnlinePaymentArgs{TenantId: tenant, Id: id, ReferenceModule: refModule, ReferenceId: refID, ReturnTo: "booking"}
}

func TestStartOnline_NoGateway(t *testing.T) {
	_, err := newEnvNoGateway(t).m.StartOnlinePayment(startArgs("p1"))
	wantErr(t, err, pm.ErrGatewayUnavailable)
}

func TestStartOnline_MissingArgs(t *testing.T) {
	e := newEnv(t)
	for _, mutate := range []func(*pm.StartOnlinePaymentArgs){
		func(a *pm.StartOnlinePaymentArgs) { a.TenantId = "" },
		func(a *pm.StartOnlinePaymentArgs) { a.Id = "" },
		func(a *pm.StartOnlinePaymentArgs) { a.ReferenceModule = "" },
		func(a *pm.StartOnlinePaymentArgs) { a.ReferenceId = "" },
	} {
		a := startArgs("p1")
		mutate(&a)
		_, err := e.m.StartOnlinePayment(a)
		wantErr(t, err, pm.ErrMissingArgs)
	}
}

func TestStartOnline_IdTaken(t *testing.T) {
	e := newEnv(t)
	e.started(t, "p1", refID)
	_, err := e.m.StartOnlinePayment(startArgs("p1"))
	wantErr(t, err, pm.ErrIdTaken)
}

func TestStartOnline_AlreadyPaid(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	_, err := e.m.StartOnlinePayment(startArgs("p2"))
	wantErr(t, err, pm.ErrAlreadyPaid)
}

func TestStartOnline_NotPayable(t *testing.T) {
	e := newEnv(t)
	e.charge.payable = false
	_, err := e.m.StartOnlinePayment(startArgs("p1"))
	wantErr(t, err, pm.ErrNotPayable)
}

func TestStartOnline_GatewayErrorMarksFailed(t *testing.T) {
	e := newEnv(t)
	e.gw.FailNext(errSimulated)
	_, err := e.m.StartOnlinePayment(startArgs("p1"))
	wantErr(t, err, pm.ErrGatewayFailed)
	if p := e.payment(t, "p1"); p.Status != pm.StatusFailed {
		t.Fatalf("status = %s, want FAILED", p.Status)
	}
}

func TestStartOnline_PendingWithToken(t *testing.T) {
	e := newEnv(t)
	res, err := e.m.StartOnlinePayment(startArgs("p1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.PaymentId != "p1" || res.Token == "" || res.RedirectUrl != "https://webpay.example/pay" {
		t.Fatalf("result = %+v", res)
	}
	p := e.payment(t, "p1")
	if p.Status != pm.StatusPending || p.Channel != pm.ChannelOnline || p.Method != pm.MethodCard || p.ProviderToken != res.Token || p.ReturnTo != "booking" {
		t.Fatalf("payment = %+v", p)
	}
}

func TestStartOnline_WithoutReturnTo(t *testing.T) {
	e := newEnv(t)
	a := startArgs("p1")
	a.ReturnTo = ""
	if _, err := e.m.StartOnlinePayment(a); err != nil {
		t.Fatalf("ReturnTo is optional: %v", err)
	}
}
