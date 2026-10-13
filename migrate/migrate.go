// Package migrate crea y actualiza el esquema de payment_manager. La app lo
// corre desde su cmd/migrate, nunca al arrancar el servidor.
package migrate

import (
	"webtyp.com/ddl"

	paymentmanager "github.com/veltylabs/payment_manager"
)

// Migrate sincroniza las tablas payment y refund.
func Migrate(conn ddl.Execer, c ddl.Compiler) error {
	return ddl.New(conn, c).Sync(&paymentmanager.Payment{}, &paymentmanager.Refund{})
}
