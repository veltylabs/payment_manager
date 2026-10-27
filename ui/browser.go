package ui

import (
	paymentmanager "github.com/veltylabs/payment_manager"
	"webtyp.com/lang"
	"webtyp.com/router"
	"webtyp.com/view"
)

const descPayments lang.Text = "Payment records, reconciliation and refunds."

// Screens declara las pantallas que este módulo ofrece al menú de la app.
//
// Uso:
//  appScreens = append(appScreens, ui.Screens(caller)...)
func Screens(caller router.Caller) []view.Screen {
	return []view.Screen{
		{
			ID:         ID,
			Label:      NavLabel,
			Icon:       ID,
			Permission: paymentmanager.ResourcePayment,
			Presenter:  paymentmanager.NewView(caller, view.WithDescription(descPayments)),
		},
	}
}
