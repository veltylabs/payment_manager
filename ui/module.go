package ui

import "webtyp.com/lang"

// ID es la identidad del módulo: prefijo RBAC en el servidor, ruta de
// navegación en el cliente. Compartido por svg.go y browser.go.
const ID = "payment_manager"

// NavLabel (no Label: html.Label ya usa ese nombre en este paquete) es el nombre de la pantalla en el menú (clave en inglés, traducida).
const NavLabel lang.Text = "Payments"
