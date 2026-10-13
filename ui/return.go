package ui

import (
	"webtyp.com/dom"

	paymentmanager "github.com/veltylabs/payment_manager"
)

// TakeReturn lee y borra de la URL el resultado que dejó la ruta de retorno de
// la pasarela (?payment=<id>&status=<estado>). ok es false si la página no
// viene de un pago. La query se borra siempre que trae status, para que
// recargar no repita la acción.
func TakeReturn() (paymentID, status string, ok bool) {
	status, hasStatus := dom.GetQuery(paymentmanager.QueryStatus)
	if !hasStatus {
		return "", "", false
	}
	paymentID, _ = dom.GetQuery(paymentmanager.QueryPayment)
	dom.ClearQuery()
	return paymentID, status, true
}
