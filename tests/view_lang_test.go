//go:build !wasm

package tests

import (
	"testing"

	"webtyp.com/lang"

	pm "github.com/veltylabs/payment_manager"
)

func TestPaymentItem_TranslatesMethodAndStatus(t *testing.T) {
	if err := lang.Load([]byte(`{"languages":["es"],"keys":{"debit":["débito"],"PAID":["Pagado"]}}`)); err != nil {
		t.Fatal(err)
	}
	lang.OutLang(lang.ES)
	defer lang.OutLang(lang.EN)
	it := (&pm.Payment{Id: "p1", Amount: 1500, Currency: pm.CurrencyCLP, Method: pm.MethodDebit, Status: pm.StatusPaid, ReferenceModule: refModule, ReferenceId: refID}).Item()
	if it.Label != "$1.500 · débito" || it.Description != "Pagado · "+refModule+" "+refID {
		t.Fatalf("item = %+v", it)
	}
}
