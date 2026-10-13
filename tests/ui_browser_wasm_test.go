//go:build wasm
package tests

import (
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
	"github.com/veltylabs/payment_manager/ui"
	"webtyp.com/dom"
	"webtyp.com/model"
)

type mockIDGen struct{}
func (mockIDGen) NewID() string { return "id" }

func TestWASM_Browser_RefundDialog(t *testing.T) {
	initTestDOM(t)
	c := &mockCaller{
		onCall: func(op string, req model.Encodable, res model.Decodable) error {
			if op == paymentmanager.ModelName+"."+paymentmanager.OpListPayments {
				// We won't simulate full decoding for the List, it's just a placeholder to check it builds
			}
			return nil
		},
	}

	mod, _ := ui.Browser(c, mockIDGen{}, "t1")
	dom.Render("app", mod.View())
}
