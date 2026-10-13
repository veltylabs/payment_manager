package tests

import (
	"testing"

	"webtyp.com/fmt"

	"webtyp.com/json"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/mock"
	"webtyp.com/storage"

	pm "github.com/veltylabs/payment_manager"
)

// opRouter monta el módulo en un router/mock con una identidad que tiene todos
// los permisos, para que cada llamada pase por la puerta de acceso real.
func opRouter(e *env) *mock.Router {
	r := &mock.Router{}
	r.Configure(mock.Config{
		Authn: func(next router.HandlerFunc) router.HandlerFunc {
			return func(c router.Context) { c.SetUserID("staff"); next(c) }
		},
		Authorize: func(string, model.Resource, model.Action) bool { return true },
	})
	e.m.MountOperations(r)
	return r
}

// call ejecuta op con el cuerpo JSON body y devuelve el estado y la respuesta.
func call(r *mock.Router, op, body string) (int, string) {
	ctx := &mock.Context{InMethod: "OP", InPath: "/" + op, InBody: []byte(body)}
	r.Invoke("OP", "/"+op, ctx)
	status := ctx.Status
	if status == 0 {
		status = 200
	}
	return status, string(ctx.ResponseBody())
}

func encode(t *testing.T, v model.Encodable) string {
	t.Helper()
	var out []byte
	if err := json.Encode(v, &out); err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestOps_RequiresTable(t *testing.T) {
	r := opRouter(newEnv(t))
	want := []struct {
		op       string
		resource string
		action   model.Action
	}{
		{pm.OpGetCharge, pm.ResourcePaymentOnline, model.Read},
		{pm.OpCreateCounterPayment, pm.ResourcePayment, model.Create},
		{pm.OpStartOnlinePayment, pm.ResourcePaymentOnline, model.Create},
		{pm.OpGetPaymentStatus, pm.ResourcePaymentOnline, model.Read},
		{pm.OpGetPayment, pm.ResourcePayment, model.Read},
		{pm.OpListPaymentsByReference, pm.ResourcePayment, model.Read},
		{pm.OpListPayments, pm.ResourcePayment, model.Read},
		{pm.OpRefundPayment, pm.ResourceRefund, model.Create},
		{pm.OpListRefunds, pm.ResourceRefund, model.Read},
		{pm.OpReconcileOnlinePayments, pm.ResourcePayment, model.Update},
	}
	routes := r.Routes()
	if len(routes) != len(want) {
		t.Fatalf("got %d routes, want %d", len(routes), len(want))
	}
	for _, w := range want {
		found := false
		for _, info := range routes {
			if info.Path == "/"+w.op {
				found = true
				if string(info.Resource) != w.resource || info.Action != w.action || info.Access != model.AccessGuarded {
					t.Errorf("%s: %s %v %v; want %s %v guarded", w.op, info.Resource, info.Action, info.Access, w.resource, w.action)
				}
				if info.Args == nil {
					t.Errorf("%s declares no args", w.op)
				}
			}
		}
		if !found {
			t.Errorf("%s not registered", w.op)
		}
	}
	if newEnv(t).m.ModelName() != pm.ModelName {
		t.Error("ModelName")
	}
}

func TestOps_HappyPaths(t *testing.T) {
	e := newEnv(t)
	r := opRouter(e)

	if st, body := call(r, pm.OpGetCharge, encode(t, &pm.GetChargeArgs{TenantId: tenant, ReferenceModule: refModule, ReferenceId: refID})); st != 200 || !fmt.Contains(body, "25000") {
		t.Errorf("get_charge: %d %s", st, body)
	}
	a := counterArgs("p1")
	if st, body := call(r, pm.OpCreateCounterPayment, encode(t, &a)); st != 200 || !fmt.Contains(body, pm.StatusPaid) {
		t.Errorf("create_counter_payment: %d %s", st, body)
	}
	s := startArgs("p2")
	s.ReferenceId = "r2"
	st, body := call(r, pm.OpStartOnlinePayment, encode(t, &s))
	if st != 200 || !fmt.Contains(body, "redirect_url") {
		t.Errorf("start_online_payment: %d %s", st, body)
	}
	if st, body := call(r, pm.OpGetPayment, encode(t, &pm.GetPaymentArgs{TenantId: tenant, Id: "p1"})); st != 200 || !fmt.Contains(body, `"p1"`) {
		t.Errorf("get_payment: %d %s", st, body)
	}
	if st, body := call(r, pm.OpListPaymentsByReference, encode(t, &pm.ListPaymentsByReferenceArgs{TenantId: tenant, ReferenceModule: refModule, ReferenceId: refID})); st != 200 || !fmt.Contains(body, `"p1"`) {
		t.Errorf("list_payments_by_reference: %d %s", st, body)
	}
	if st, body := call(r, pm.OpListPayments, encode(t, &pm.ListPaymentsArgs{TenantId: tenant, From: 1, To: nowSec() + 60})); st != 200 || !fmt.Contains(body, `"p2"`) {
		t.Errorf("list_payments: %d %s", st, body)
	}
	f := refundArgs("f1", "p1", 100)
	if st, body := call(r, pm.OpRefundPayment, encode(t, &f)); st != 200 || !fmt.Contains(body, pm.RefundKindManual) {
		t.Errorf("refund_payment: %d %s", st, body)
	}
	if st, body := call(r, pm.OpListRefunds, encode(t, &pm.ListRefundsArgs{TenantId: tenant, PaymentId: "p1"})); st != 200 || !fmt.Contains(body, `"f1"`) {
		t.Errorf("list_refunds: %d %s", st, body)
	}
	if st, body := call(r, pm.OpReconcileOnlinePayments, encode(t, &pm.ReconcileOnlinePaymentsArgs{TenantId: tenant})); st != 200 || !fmt.Contains(body, "closed") {
		t.Errorf("reconcile_online_payments: %d %s", st, body)
	}
}

func TestOps_NoProviderTokenInResponses(t *testing.T) {
	e := newEnv(t)
	e.paidOnline(t, "p1", refID)
	r := opRouter(e)
	for _, c := range []struct{ op, body string }{
		{pm.OpGetPayment, encode(t, &pm.GetPaymentArgs{TenantId: tenant, Id: "p1"})},
		{pm.OpListPaymentsByReference, encode(t, &pm.ListPaymentsByReferenceArgs{TenantId: tenant, ReferenceModule: refModule, ReferenceId: refID})},
		{pm.OpListPayments, encode(t, &pm.ListPaymentsArgs{TenantId: tenant, From: 1, To: nowSec() + 60})},
	} {
		st, body := call(r, c.op, c.body)
		if st != 200 || fmt.Contains(body, "provider_token") || fmt.Contains(body, "sbx-") {
			t.Errorf("%s leaks the gateway token: %d %s", c.op, st, body)
		}
	}
	a := counterArgs("p9")
	a.ReferenceId = "r9"
	if _, body := call(r, pm.OpCreateCounterPayment, encode(t, &a)); fmt.Contains(body, "provider_token") {
		t.Errorf("create_counter_payment leaks the token: %s", body)
	}
}

func TestOps_PaymentStatusShape(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	st, body := call(opRouter(e), pm.OpGetPaymentStatus, encode(t, &pm.GetPaymentArgs{TenantId: tenant, Id: "p1"}))
	if st != 200 || body != `{"id":"p1","status":"PAID"}` {
		t.Fatalf("get_payment_status: %d %s", st, body)
	}
}

func TestOps_BadBodyAndInvalidArgs(t *testing.T) {
	r := opRouter(newEnv(t))
	ops := []string{pm.OpGetCharge, pm.OpCreateCounterPayment, pm.OpStartOnlinePayment, pm.OpGetPaymentStatus, pm.OpGetPayment,
		pm.OpListPaymentsByReference, pm.OpListPayments, pm.OpRefundPayment, pm.OpListRefunds, pm.OpReconcileOnlinePayments}
	for _, op := range ops {
		if st, _ := call(r, op, `{`); st != 400 {
			t.Errorf("%s with a broken body: %d, want 400", op, st)
		}
		if st, _ := call(r, op, `{}`); st != 400 {
			t.Errorf("%s with missing required args: %d, want 400", op, st)
		}
	}
}

// Cada op lleva sus errores al código de la tabla; uno por op.
func TestOps_ErrorStatusTable(t *testing.T) {
	e := newEnv(t)
	r := opRouter(e)
	e.paidCounter(t, "p1", refID)
	cases := []struct {
		op, body string
		status   int
	}{
		{pm.OpGetCharge, encode(t, &pm.GetChargeArgs{TenantId: tenant, ReferenceModule: "otro", ReferenceId: refID}), 400},
		{pm.OpCreateCounterPayment, encode(t, &pm.CreateCounterPaymentArgs{TenantId: tenant, Id: "p2", ReferenceModule: refModule, ReferenceId: refID, Method: pm.MethodCash}), 409},
		{pm.OpStartOnlinePayment, encode(t, &pm.StartOnlinePaymentArgs{TenantId: tenant, Id: "p1", ReferenceModule: refModule, ReferenceId: "r2"}), 409},
		{pm.OpGetPaymentStatus, encode(t, &pm.GetPaymentArgs{TenantId: tenant, Id: "nope"}), 404},
		{pm.OpGetPayment, encode(t, &pm.GetPaymentArgs{TenantId: tenant, Id: "nope"}), 404},
		{pm.OpRefundPayment, encode(t, &pm.RefundPaymentArgs{TenantId: tenant, Id: "f1", PaymentId: "p1", Amount: 999999, Reason: "no asistió"}), 400},
	}
	for _, c := range cases {
		if st, body := call(r, c.op, c.body); st != c.status || body == "" {
			t.Errorf("%s: %d %q, want %d with the error text", c.op, st, body, c.status)
		}
	}

	e.conn.failOn(tPayment, storage.ActionReadAll, 0)
	for _, c := range []struct{ op, body string }{
		{pm.OpListPaymentsByReference, encode(t, &pm.ListPaymentsByReferenceArgs{TenantId: tenant, ReferenceModule: refModule, ReferenceId: refID})},
		{pm.OpListPayments, encode(t, &pm.ListPaymentsArgs{TenantId: tenant, From: 1, To: 2})},
		{pm.OpReconcileOnlinePayments, encode(t, &pm.ReconcileOnlinePaymentsArgs{TenantId: tenant})},
	} {
		if st, _ := call(r, c.op, c.body); st != 500 {
			t.Errorf("%s with a storage error: %d, want 500", c.op, st)
		}
	}
	e.conn.failOn(tRefund, storage.ActionReadAll, 0)
	if st, _ := call(r, pm.OpListRefunds, encode(t, &pm.ListRefundsArgs{TenantId: tenant, PaymentId: "p1"})); st != 500 {
		t.Errorf("list_refunds with a storage error: %d, want 500", st)
	}
}

func TestStatusFor(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{pm.ErrNotFound, 404}, {pm.ErrRefundNotFound, 404},
		{pm.ErrMissingArgs, 400}, {pm.ErrUnknownReference, 400}, {pm.ErrNotPayable, 400}, {pm.ErrUnsupportedCurrency, 400},
		{pm.ErrInvalidMethod, 400}, {pm.ErrInvalidTransition, 400}, {pm.ErrRefundExceeds, 400},
		{pm.ErrIdTaken, 409}, {pm.ErrAlreadyPaid, 409}, {pm.ErrNotPaid, 409}, {pm.ErrConflict, 409},
		{pm.ErrGatewayUnavailable, 500}, {pm.ErrGatewayFailed, 500}, {pm.ErrAmountMismatch, 500}, {errSimulated, 500},
	}
	for _, c := range cases {
		if got := pm.StatusFor(c.err); got != c.code {
			t.Errorf("StatusFor(%v) = %d, want %d", c.err, got, c.code)
		}
		if c.err.Error() == "" {
			t.Errorf("%v has no text", c.err)
		}
	}
}

// Sin identidad, la puerta niega: las ops no son públicas por sí solas.
func TestOps_DeniedWithoutIdentity(t *testing.T) {
	r := &mock.Router{}
	newEnv(t).m.MountOperations(r)
	ctx := &mock.Context{InMethod: "OP", InBody: []byte(`{}`)}
	r.Invoke("OP", "/"+pm.OpGetCharge, ctx)
	if ctx.Status != 403 {
		t.Fatalf("status %d, want 403", ctx.Status)
	}
}

var _ router.OperationModule = (*pm.Module)(nil)
