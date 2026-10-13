package tests

import (
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
)

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		amount   int64
		currency string
		expected string
	}{
		{25000, "CLP", "$25.000"},
		{1234567, "CLP", "$1.234.567"},
		{0, "CLP", "$0"},
		{1250, "USD", "US$12,50"},
		{100, "EUR", "100 EUR"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := paymentmanager.FormatAmount(tt.amount, tt.currency)
			if got != tt.expected {
				t.Errorf("FormatAmount(%d, %q) = %q, want %q", tt.amount, tt.currency, got, tt.expected)
			}
		})
	}
}
