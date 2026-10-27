//go:build browser

package tests

import (
	"syscall/js"
	"testing"

	"webtyp.com/dom"
	"webtyp.com/fmt"
	"webtyp.com/router"
	"webtyp.com/router/loopback"

	pm "github.com/veltylabs/payment_manager"
	"github.com/veltylabs/payment_manager/ui"
)

func TestMain(m *testing.M) {
	doc := js.Global().Get("document")
	app := doc.Call("createElement", "div")
	app.Set("id", "app")
	doc.Get("body").Call("appendChild", app)
	m.Run()
}

// seqIDs genera ids "ui-1", "ui-2", …
type seqIDs struct{ n int }

func (s *seqIDs) NewID() string { s.n++; return "ui-" + fmt.Convert(s.n).String() }

// inputEvent es un evento "input" que burbujea, como el que dispara el usuario.
func inputEvent() js.Value {
	init := js.Global().Get("Object").New()
	init.Set("bubbles", true)
	return js.Global().Get("Event").New("input", init)
}

func app() js.Value { return js.Global().Get("document").Call("getElementById", "app") }

func mount(t *testing.T, c dom.Component) js.Value {
	t.Helper()
	app().Set("innerHTML", "")
	if err := dom.Render("app", c); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return app()
}

func text(el js.Value) string { return el.Get("textContent").String() }

// buttonByText devuelve el primer <button> cuyo texto es label.
func buttonByText(t *testing.T, root js.Value, label string) js.Value {
	t.Helper()
	buttons := root.Call("querySelectorAll", "button")
	for i := 0; i < buttons.Length(); i++ {
		if fmt.Convert(text(buttons.Index(i))).TrimSpace().String() == label {
			return buttons.Index(i)
		}
	}
	t.Fatalf("no button %q in %s", label, root.Get("innerHTML").String())
	return js.Null()
}

func step(caller router.Caller, channel string) *ui.PaymentStep {
	return &ui.PaymentStep{Caller: caller, IDs: &seqIDs{}, TenantID: tenant, Channel: channel, ReferenceModule: refModule, ReferenceID: refID, ReturnTo: "booking"}
}

func TestWASM_PaymentStep_ChargeErrorVisible(t *testing.T) {
	root := mount(t, step(errCaller{}, pm.ChannelCounter))
	if !fmt.Contains(text(root), errSimulated.Error()) {
		t.Fatalf("the get_charge error must be visible: %s", text(root))
	}
}

func TestWASM_PaymentStep_NothingToPay(t *testing.T) {
	e := newEnv(t)
	e.charge.payable = false
	root := mount(t, step(loopback.WithTenant(tenant, e.m), pm.ChannelCounter))
	if !fmt.Contains(text(root), "Nothing to pay") || root.Call("querySelector", "button").Truthy() {
		t.Fatalf("got %s", root.Get("innerHTML").String())
	}
}

func TestWASM_PaymentStep_CounterRegistersAndCallsOnPaid(t *testing.T) {
	e := newEnv(t)
	s := step(loopback.WithTenant(tenant, e.m), pm.ChannelCounter)
	var paid string
	s.OnPaid = func(id string) { paid = id }
	root := mount(t, s)
	if !fmt.Contains(text(root), "$25.000") {
		t.Fatalf("the amount must show: %s", text(root))
	}
	buttonByText(t, root, "Debit").Call("click")
	buttonByText(t, root, "Register payment").Call("click")

	if paid != "ui-1" {
		t.Fatalf("OnPaid = %q, want the generated id ui-1", paid)
	}
	p := e.payment(t, "ui-1")
	if p.Method != pm.MethodDebit || p.ReceiptNumber != "R-000001" || p.Status != pm.StatusPaid {
		t.Fatalf("payment = %+v", p)
	}
}

func TestWASM_PaymentStep_TerminalRegistersAndCallsOnPaid(t *testing.T) {
	e := newEnvTerminal(t)
	s := step(loopback.WithTenant(tenant, e.m), pm.ChannelCounter)
	var paid string
	s.OnPaid = func(id string) { paid = id }
	root := mount(t, s)
	if !fmt.Contains(text(root), "$25.000") {
		t.Fatalf("the amount must show: %s", text(root))
	}
	buttonByText(t, root, "Debit").Call("click")
	buttonByText(t, root, "Register payment").Call("click")

	p := e.payment(t, "ui-1")
	if p.Channel != pm.ChannelTerminal || p.Status != pm.StatusPending {
		t.Fatalf("payment channel/status = %s/%s", p.Channel, p.Status)
	}

	e.term.Approve(p.ProviderToken)

	// loop wait
	for i := 0; i < 25; i++ {
		if paid != "" {
			break
		}
		// block loop back tick
		ch := make(chan struct{})
		js.Global().Call("setTimeout", js.FuncOf(func(this js.Value, args []js.Value) any {
			close(ch)
			return nil
		}), 200)
		<-ch
	}

	if paid != "ui-1" {
		t.Fatalf("OnPaid = %q, want ui-1", paid)
	}
}

func TestWASM_PaymentStep_CounterErrorVisible(t *testing.T) {
	e := newEnv(t)
	e.charge.err = errSimulated
	root := mount(t, step(loopback.WithTenant(tenant, e.m), pm.ChannelCounter))
	if !fmt.Contains(text(root), errSimulated.Error()) {
		t.Fatalf("the error must be visible: %s", text(root))
	}
}

func TestWASM_PaymentStep_OnlineErrorVisible(t *testing.T) {
	e := newEnvNoGateway(t)
	root := mount(t, step(loopback.WithTenant(tenant, e.m), pm.ChannelOnline))
	buttonByText(t, root, "Pay online").Call("click")
	if !fmt.Contains(text(root), pm.ErrGatewayUnavailable.Error()) {
		t.Fatalf("the error must be visible: %s", text(root))
	}
}

func TestWASM_PaymentStep_OnlineNavigatesPost(t *testing.T) {
	e := newEnv(t)
	proto := js.Global().Get("HTMLFormElement").Get("prototype")
	original := proto.Get("submit")
	var action, token string
	fake := js.FuncOf(func(this js.Value, _ []js.Value) any {
		action = this.Get("action").String()
		token = this.Call("querySelector", "input[name='"+"token_ws"+"']").Get("value").String()
		return nil
	})
	proto.Set("submit", fake)
	defer func() { proto.Set("submit", original); fake.Release() }()

	root := mount(t, step(loopback.WithTenant(tenant, e.m), pm.ChannelOnline))
	buttonByText(t, root, "Pay online").Call("click")

	if action != "https://webpay.example/pay" || !fmt.HasPrefix(token, "sbx-") {
		t.Fatalf("POST to %q with token %q", action, token)
	}
	if p := e.payment(t, "ui-1"); p.Status != pm.StatusPending || p.ReturnTo != "booking" {
		t.Fatalf("payment = %+v", p)
	}
}

func setURL(url string) {
	js.Global().Get("history").Call("replaceState", js.Null(), "", url)
}

func TestWASM_TakeReturn_ReadsAndClears(t *testing.T) {
	setURL("/?" + pm.QueryPayment + "=p1&" + pm.QueryStatus + "=PAID#booking")
	id, status, ok := ui.TakeReturn()
	if !ok || id != "p1" || status != pm.StatusPaid {
		t.Fatalf("got %q %q %v", id, status, ok)
	}
	if loc := js.Global().Get("location"); loc.Get("search").String() != "" || loc.Get("hash").String() != "#booking" {
		t.Fatalf("query not cleared: %s%s", loc.Get("search").String(), loc.Get("hash").String())
	}
}

func TestWASM_TakeReturn_NoQuery(t *testing.T) {
	setURL("/#booking")
	if _, _, ok := ui.TakeReturn(); ok {
		t.Fatal("no status in the URL: ok must be false")
	}
}

func TestWASM_PaymentsBrowser_RefundAndReconcile(t *testing.T) {
	e := newEnv(t)
	e.paidCounter(t, "p1", refID)
	v := ui.NewPaymentsView(loopback.WithTenant(tenant, e.m), &seqIDs{}, tenant)
	v.Init(nil)
	root := mount(t, v)

	rows := root.Call("querySelectorAll", ".targetlist__row")
	if rows.Length() != 1 || !fmt.Contains(text(rows.Index(0)), "$25.000") {
		t.Fatalf("rows = %d: %s", rows.Length(), root.Get("innerHTML").String())
	}
	rows.Index(0).Call("click")

	amount := root.Call("querySelector", "input[name='refund-amount']")
	if amount.Get("value").String() != "25000" {
		t.Fatalf("the refund amount defaults to the balance, got %q", amount.Get("value").String())
	}
	setValue := func(el js.Value, v string) {
		el.Set("value", v)
		el.Call("dispatchEvent", inputEvent())
	}
	setValue(amount, "5000")
	setValue(root.Call("querySelector", "textarea[name='refund-reason']"), "no asistió")
	buttonByText(t, root, "Refund").Call("click")

	if p := e.payment(t, "p1"); p.RefundedAmount != 5000 || p.Status != pm.StatusPartiallyRefunded {
		t.Fatalf("payment = %+v; screen: %s", p, text(root))
	}
	if !fmt.Contains(text(root), "no asistió") {
		t.Fatalf("the detail lists the new refund: %s", text(root))
	}

	setValue(root.Call("querySelector", "input[name='refund-amount']"), "1e3")
	buttonByText(t, root, "Refund").Call("click")
	if !fmt.Contains(text(root), "Invalid amount") {
		t.Fatalf("an invalid amount shows a message: %s", text(root))
	}
	setValue(root.Call("querySelector", "textarea[name='refund-reason']"), "devolución total")
	setValue(root.Call("querySelector", "input[name='refund-amount']"), "999999")
	buttonByText(t, root, "Refund").Call("click")
	if !fmt.Contains(text(root), pm.ErrRefundExceeds.Error()) {
		t.Fatalf("a refund error is visible: %s", text(root))
	}

	buttonByText(t, root, "Reconcile payments").Call("click")
	if !fmt.Contains(text(root), "Closed payments: 0") {
		t.Fatalf("reconcile reports what it closed: %s", text(root))
	}
}

func TestWASM_PaymentsBrowser_ErrorsVisible(t *testing.T) {
	v := ui.NewPaymentsView(errCaller{}, &seqIDs{}, tenant)
	v.Init(nil)
	root := mount(t, v)
	if !fmt.Contains(text(root), errSimulated.Error()) {
		t.Fatalf("a list error is visible: %s", text(root))
	}
	buttonByText(t, root, "Reconcile payments").Call("click")
	if !fmt.Contains(text(root), errSimulated.Error()) {
		t.Fatalf("a reconcile error is visible: %s", text(root))
	}
}
