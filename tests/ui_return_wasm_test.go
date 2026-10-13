//go:build wasm
package tests

import (
	"testing"
	"github.com/veltylabs/payment_manager/ui"
	"webtyp.com/dom"
	"syscall/js"
)

func TestWASM_TakeReturn_ReadsAndClears(t *testing.T) {
	initTestDOM(t)
	// mock window URL search manually
	js.Global().Get("window").Get("history").Call("pushState", js.Null(), "", "?token_ws=tok1")

	called := ""
	v := ui.NewTakeReturn(func(token string) { called = token })
	dom.Render("app", v.Render())

	if called != "tok1" {
		t.Errorf("expected tok1, got %q", called)
	}

	// ensure query cleared
	loc := js.Global().Get("window").Get("location").Get("search").String()
	if loc != "" {
		t.Errorf("expected empty search, got %q", loc)
	}
}
