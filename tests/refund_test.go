package tests

import (
	"testing"

	"webtyp.com/orm"

	pm "github.com/veltylabs/payment_manager"
)

func refundArgs(id, paymentID string, amount int64) pm.RefundPaymentArgs {
	return pm.RefundPaymentArgs{TenantId: tenant, Id: id, PaymentId: paymentID, Amount: amount, Reason: "el paciente no asistió", Method: pm.MethodCash}
}

func TestRefund_MissingArgs(t *testing.T) {
	e := newEnv(t)
	for _, mutate := range []func(*pm.RefundPaymentArgs){
		func(a *pm.RefundPaymentArgs) { a.TenantId = "" },
		func(a *pm.RefundPaymentArgs) { a.Id = "" },
		func(a *pm.RefundPaymentArgs) { a.PaymentId = "" },
		func(a *pm.RefundPaymentArgs) { a.Reason = "" },
	} {
		a := refundArgs("f1", "p1", 100)
		mutate(&a)
		_, err := e.m.RefundPayment(a)
		wantErr(t, err, pm.ErrMissingArgs)
	}
}

func TestRefund_RetryReturnsStored(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	first, err := e.m.RefundPayment(refundArgs("f1", "p1", 5000))
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.m.RefundPayment(refundArgs("f1", "p1", 5000))
	if err != nil || again.Id != first.Id {
		t.Fatalf("retry = %+v, %v", again, err)
	}
	if p := e.payment(t, "p1"); p.RefundedAmount != 5000 {
		t.Fatalf("a retry must not refund twice: refunded %d", p.RefundedAmount)
	}
}

func TestRefund_IdTaken(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	e.paidCounter(t, "p2", "r2")
	if _, err := e.m.RefundPayment(refundArgs("f1", "p1", 100)); err != nil {
		t.Fatal(err)
	}
	_, err := e.m.RefundPayment(refundArgs("f1", "p2", 100))
	wantErr(t, err, pm.ErrIdTaken)
}

func TestRefund_OtherTenantNotFound(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	a := refundArgs("f1", "p1", 100)
	a.TenantId = "t2"
	_, err := e.m.RefundPayment(a)
	wantErr(t, err, pm.ErrNotFound)
}

func TestRefund_PendingPaymentRejected(t *testing.T) {
	e := newEnv(t)
	e.started(t, "p1", refID)
	_, err := e.m.RefundPayment(refundArgs("f1", "p1", 100))
	wantErr(t, err, pm.ErrInvalidTransition)
}

func TestRefund_ExceedsBalance(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	_, err := e.m.RefundPayment(refundArgs("f1", "p1", 25001))
	wantErr(t, err, pm.ErrRefundExceeds)
}

func TestRefund_ZeroAmount(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	_, err := e.m.RefundPayment(refundArgs("f1", "p1", 0))
	wantErr(t, err, pm.ErrRefundExceeds)
}

func TestRefund_CounterInvalidMethod(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	a := refundArgs("f1", "p1", 100)
	a.Method = pm.MethodWebpay
	_, err := e.m.RefundPayment(a)
	wantErr(t, err, pm.ErrInvalidMethod)
}

func TestRefund_PartialThenPartial(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	for i, id := range []string{"f1", "f2"} {
		ref, err := e.m.RefundPayment(refundArgs(id, "p1", 5000))
		if err != nil || ref.Kind != pm.RefundKindManual || ref.Method != pm.MethodCash {
			t.Fatalf("refund %d = %+v, %v", i, ref, err)
		}
	}
	p := e.payment(t, "p1")
	if p.Status != pm.StatusPartiallyRefunded || p.RefundedAmount != 10000 || p.Revision != 3 {
		t.Fatalf("payment = %+v", p)
	}
}

func TestRefund_RestMakesRefunded(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	if _, err := e.m.RefundPayment(refundArgs("f1", "p1", 5000)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.m.RefundPayment(refundArgs("f2", "p1", 20000)); err != nil {
		t.Fatal(err)
	}
	if p := e.payment(t, "p1"); p.Status != pm.StatusRefunded || p.RefundedAmount != 25000 {
		t.Fatalf("payment = %+v", p)
	}
}

func TestRefund_PublishesRefunded(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	if _, err := e.m.RefundPayment(refundArgs("f1", "p1", 25000)); err != nil {
		t.Fatal(err)
	}
	got := e.pub.topics()
	if len(got) != 2 || got[1] != pm.EventPaymentRefunded {
		t.Fatalf("events = %v", got)
	}
	if _, ok := e.pub.events[1].Payload.(*pm.Refund); !ok {
		t.Fatalf("payment.refunded carries a *Refund, got %T", e.pub.events[1].Payload)
	}
}

func TestRefund_OnlineRecordsGatewayKind(t *testing.T) {
	e := newEnv(t)
	token := e.paidOnline(t, "p1", refID)
	full, err := e.m.RefundPayment(refundArgs("f1", "p1", 25000))
	if err != nil || full.Kind != pm.RefundKindReversed || full.AuthorizationCode == "" || e.gw.Refunded(token) != 25000 {
		t.Fatalf("full refund = %+v, %v", full, err)
	}

	token2 := e.paidOnline(t, "p2", "r2")
	part, err := e.m.RefundPayment(refundArgs("f2", "p2", 1000))
	if err != nil || part.Kind != pm.RefundKindNullified || e.gw.Refunded(token2) != 1000 {
		t.Fatalf("partial refund = %+v, %v", part, err)
	}
}

func TestRefund_GatewayErrorWritesNothing(t *testing.T) {
	e := newEnv(t)
	e.paidOnline(t, "p1", refID)
	e.hook.refundErr = errSimulated
	_, err := e.m.RefundPayment(refundArgs("f1", "p1", 1000))
	wantErr(t, err, pm.ErrGatewayFailed)
	if p := e.payment(t, "p1"); p.RefundedAmount != 0 || p.Status != pm.StatusPaid {
		t.Fatalf("payment changed: %+v", p)
	}
	if refunds, _ := e.m.ListRefunds(tenant, "p1"); len(refunds) != 0 {
		t.Fatalf("refunds = %v", refunds)
	}
}

// Otra devolución se guarda entre la lectura del pago y la escritura: ErrConflict
// y la segunda no se escribe.
func TestRefund_ConcurrentRevisionConflict(t *testing.T) {
	e := newEnv(t)
	e.paidOnline(t, "p1", refID)
	e.hook.onRefund = func() {
		if _, err := e.m.RefundPayment(refundArgs("f-otra", "p1", 1000)); err != nil {
			t.Fatalf("concurrent refund: %v", err)
		}
	}
	_, err := e.m.RefundPayment(refundArgs("f1", "p1", 2000))
	wantErr(t, err, pm.ErrConflict)
	if p := e.payment(t, "p1"); p.RefundedAmount != 1000 {
		t.Fatalf("refunded = %d, want only the concurrent 1000", p.RefundedAmount)
	}
	if refunds, _ := e.m.ListRefunds(tenant, "p1"); len(refunds) != 1 {
		t.Fatalf("refunds = %d, want 1 (rolled back)", len(refunds))
	}
}

func TestRefund_OnlineWithoutGateway(t *testing.T) {
	e := newEnv(t)
	e.paidOnline(t, "p1", refID)
	noGateway, err := pm.New(orm.New(e.conn), pm.Deps{Charges: []pm.ChargeSource{{Module: refModule, Reader: e.charge}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = noGateway.RefundPayment(refundArgs("f1", "p1", 100))
	wantErr(t, err, pm.ErrGatewayUnavailable)
}

func TestRefund_ReasonTooLongFailsValidation(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	a := refundArgs("f1", "p1", 100)
	long := make([]byte, 256)
	for i := range long {
		long[i] = 'a'
	}
	a.Reason = string(long)
	if _, err := e.m.RefundPayment(a); err == nil {
		t.Fatal("a reason over 255 characters must fail validation")
	}
}
