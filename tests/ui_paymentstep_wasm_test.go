//go:build wasm
package tests

import (
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
	"github.com/veltylabs/payment_manager/ui"
	"webtyp.com/dom"
	"syscall/js"
	"webtyp.com/model"
)

type mockCaller struct {
	onCall func(op string, req model.Encodable, res model.Decodable) error
}
func (m *mockCaller) Call(op string, req model.Encodable, res model.Decodable, done func(error)) {
	err := m.onCall(op, req, res)
	done(err)
}
func (m *mockCaller) Dispatch(action string, payload model.Encodable) {}

func initTestDOM(t *testing.T) {
	doc := js.Global().Get("document")
	if doc.IsUndefined() {
		t.Fatal("no document")
	}
	body := doc.Get("body")
	body.Set("innerHTML", `<div id="app"></div>`)
}

func TestWASM_PaymentStep_ChargeErrorVisible(t *testing.T) {
	initTestDOM(t)
	c := &mockCaller{
		onCall: func(op string, req model.Encodable, res model.Decodable) error {
			return paymentmanager.ErrGatewayFailed
		},
	}

	v := ui.NewPaymentStep(c, "t1", "mod", "ref1", nil)
	dom.Render("app", v)

	doc := js.Global().Get("document")
	app := doc.Call("getElementById", "app")
	if app.Get("textContent").String() != "Error: gateway failed" {
		t.Errorf("expected error message, got %q", app.Get("textContent").String())
	}
}

func TestWASM_PaymentStep_NothingToPay(t *testing.T) {
	initTestDOM(t)
	c := &mockCaller{
		onCall: func(op string, req model.Encodable, res model.Decodable) error {
			out := res.(*paymentmanager.ChargeResult)
			out.Amount = 0
			return nil
		},
	}

	called := false
	v := ui.NewPaymentStep(c, "t1", "mod", "ref1", func(){ called = true })
	dom.Render("app", v)

	if !called {
		t.Error("OnPaid not called")
	}
}

func TestWASM_PaymentStep_CounterRegistersAndCallsOnPaid(t *testing.T) {
	initTestDOM(t)
	c := &mockCaller{
		onCall: func(op string, req model.Encodable, res model.Decodable) error {
			if op == paymentmanager.ModelName+"."+paymentmanager.OpGetCharge {
				out := res.(*paymentmanager.ChargeResult)
				out.Amount = 25000
				out.Currency = "CLP"
			}
			return nil
		},
	}

	called := false
	v := ui.NewPaymentStep(c, "t1", "mod", "ref1", func(){ called = true })
	dom.Render("app", v)

	doc := js.Global().Get("document")
	app := doc.Call("getElementById", "app")

	// mock a click
	btns := app.Call("getElementsByTagName", "button")
	var payBtn js.Value
	for i := 0; i < btns.Length(); i++ {
		if btns.Index(i).Get("textContent").String() == "Pagar" {
			payBtn = btns.Index(i)
		}
	}
	if payBtn.IsUndefined() { t.Fatal("Pagar not found") }

	payBtn.Call("click")
	if !called {
		t.Error("OnPaid not called")
	}
}
