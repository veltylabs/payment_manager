//go:build browser

package ui

import (
	"webtyp.com/lang"
	"webtyp.com/layout/platformd"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/svg"
)

// Browser construye la pantalla Pagos: los pagos de los últimos 30 días, el
// detalle del elegido con sus devoluciones, devolver y conciliar.
func Browser(caller router.Caller, ids model.IDGenerator, tenantID string) (platformd.UIModule, error) {
	return platformd.NewUIModule(ID, lang.Translate(NavLabel).String(), svg.Icon(ID), NewPaymentsView(caller, ids, tenantID)), nil
}
