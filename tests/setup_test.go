package tests

import (
	"testing"

	"webtyp.com/events"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/storage"
	"webtyp.com/storage/mem"

	pm "github.com/veltylabs/payment_manager"
	"github.com/veltylabs/payment_manager/sandbox"
)

const (
	tenant    = "t1"
	refModule = "appointment_booking"
	refID     = "r1"
	returnURL = "https://clinic.example/payments/webpay/return"
)

var errSimulated = fmt.Err("error simulado")

// fakeCharge es el ChargeReader del módulo dueño de la referencia.
type fakeCharge struct {
	amount   int64
	currency string
	payable  bool
	err      error
}

func (f *fakeCharge) Charge(tenantId, referenceId string) (int64, string, bool, error) {
	return f.amount, f.currency, f.payable, f.err
}

// recorder guarda los eventos publicados.
type recorder struct{ events []events.Event }

func (r *recorder) Publish(e events.Event) { r.events = append(r.events, e) }

func (r *recorder) topics() []string {
	out := make([]string, len(r.events))
	for i, e := range r.events {
		out[i] = e.Topic
	}
	return out
}

// failingConn envuelve storage/mem y hace fallar una tabla y una acción a
// pedido. Compile pasa por toda lectura y escritura, dentro o fuera de Tx.
type failingConn struct {
	storage.Conn
	table  string
	action storage.Action
	after  int // cuántas coincidencias dejar pasar antes de fallar
	seen   int
}

func (f *failingConn) Compile(q storage.Query, m model.Model) (storage.Plan, error) {
	if f.table != "" && q.Table == f.table && q.Action == f.action {
		f.seen++
		if f.seen > f.after {
			return storage.Plan{}, errSimulated
		}
	}
	return f.Conn.Compile(q, m)
}

// BeginTx se reenvía: incrustar storage.Conn lo ocultaría y orm.Tx fallaría.
func (f *failingConn) BeginTx() (storage.TxBoundExecutor, error) {
	return f.Conn.(storage.TxExecutor).BeginTx()
}

// failOn hace fallar desde ahora la acción sobre la tabla, después de skip coincidencias.
func (f *failingConn) failOn(table string, action storage.Action, skip int) {
	f.table, f.action, f.after, f.seen = table, action, skip, 0
}

// hookGateway envuelve el sandbox para meter, entre la lectura de un pago y su
// escritura, lo que haría otro proceso al mismo tiempo (onCommit, onStatus,
// onRefund), o para hacer fallar solo las devoluciones (refundErr).
type hookGateway struct {
	*sandbox.Gateway
	onCommit, onStatus, onRefund func()
	refundErr                    error
	// commitResult, si no es nil, reemplaza lo que informa Commit (datos raros
	// de la pasarela).
	commitResult *pm.GatewayResult
}

func (h *hookGateway) Commit(token string) (pm.GatewayResult, error) {
	run(&h.onCommit)
	if h.commitResult != nil {
		return *h.commitResult, nil
	}
	return h.Gateway.Commit(token)
}

func (h *hookGateway) Status(token string) (pm.GatewayResult, error) {
	run(&h.onStatus)
	return h.Gateway.Status(token)
}

func (h *hookGateway) Refund(token string, amount int64) (pm.GatewayRefund, error) {
	run(&h.onRefund)
	if h.refundErr != nil {
		return pm.GatewayRefund{}, h.refundErr
	}
	return h.Gateway.Refund(token, amount)
}

// run ejecuta el gancho una sola vez (lo borra antes, para que una llamada
// anidada no lo repita).
func run(hook *func()) {
	if fn := *hook; fn != nil {
		*hook = nil
		fn()
	}
}

type env struct {
	m      *pm.Module
	gw     *sandbox.Gateway
	hook   *hookGateway
	pub    *recorder
	charge *fakeCharge
	conn   *failingConn
}

func newEnv(t *testing.T) *env {
	t.Helper()
	return build(t, true)
}

func newEnvNoGateway(t *testing.T) *env {
	t.Helper()
	return build(t, false)
}

func build(t *testing.T, withGateway bool) *env {
	t.Helper()
	e := &env{
		pub:    &recorder{},
		charge: &fakeCharge{amount: 25000, currency: pm.CurrencyCLP, payable: true},
		conn:   &failingConn{Conn: mem.New()},
	}
	deps := pm.Deps{Publisher: e.pub, Charges: []pm.ChargeSource{{Module: refModule, Reader: e.charge}}}
	if withGateway {
		e.gw = sandbox.New("https://webpay.example/pay")
		e.hook = &hookGateway{Gateway: e.gw}
		deps.Gateway = e.hook
		deps.ReturnURL = returnURL
	}
	m, err := pm.New(orm.New(e.conn), deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	e.m = m
	return e
}

// paidCounter registra un pago de mesón pagado de 25000 CLP.
func (e *env) paidCounter(t *testing.T, id, ref string) pm.Payment {
	t.Helper()
	p, err := e.m.CreateCounterPayment(pm.CreateCounterPaymentArgs{TenantId: tenant, Id: id, ReferenceModule: refModule, ReferenceId: ref, Method: pm.MethodCash})
	if err != nil {
		t.Fatalf("CreateCounterPayment: %v", err)
	}
	return p
}

// started inicia un pago online y devuelve su token.
func (e *env) started(t *testing.T, id, ref string) string {
	t.Helper()
	res, err := e.m.StartOnlinePayment(pm.StartOnlinePaymentArgs{TenantId: tenant, Id: id, ReferenceModule: refModule, ReferenceId: ref, ReturnTo: "appointment_booking_online"})
	if err != nil {
		t.Fatalf("StartOnlinePayment: %v", err)
	}
	return res.Token
}

// paidOnline deja un pago online pagado y devuelve su token.
func (e *env) paidOnline(t *testing.T, id, ref string) string {
	t.Helper()
	token := e.started(t, id, ref)
	e.gw.Approve(token)
	if _, err := e.m.CompleteOnlinePayment(pm.OnlineReturn{Token: token}); err != nil {
		t.Fatalf("CompleteOnlinePayment: %v", err)
	}
	return token
}

func (e *env) payment(t *testing.T, id string) pm.Payment {
	t.Helper()
	p, err := e.m.GetPayment(tenant, id)
	if err != nil {
		t.Fatalf("GetPayment(%s): %v", id, err)
	}
	return p
}

func wantErr(t *testing.T, got, want error) {
	t.Helper()
	if got != want {
		t.Fatalf("err = %v, want %v", got, want)
	}
}
