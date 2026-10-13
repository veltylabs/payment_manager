package tests

import (
	"testing"

	pm "github.com/veltylabs/payment_manager"
)

func TestFormatAmount(t *testing.T) {
	cases := []struct {
		amount   int64
		currency string
		want     string
	}{
		{25000, pm.CurrencyCLP, "$25.000"},
		{1234567, pm.CurrencyCLP, "$1.234.567"},
		{0, pm.CurrencyCLP, "$0"},
		{999, pm.CurrencyCLP, "$999"},
		{1250, pm.CurrencyUSD, "US$12,50"},
		{5, pm.CurrencyUSD, "US$0,05"},
		{700, "EUR", "700 EUR"},
	}
	for _, c := range cases {
		if got := pm.FormatAmount(c.amount, c.currency); got != c.want {
			t.Errorf("FormatAmount(%d, %s) = %q, want %q", c.amount, c.currency, got, c.want)
		}
	}
}
