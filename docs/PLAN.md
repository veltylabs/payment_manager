---
PLAN: "feat: migrate ui to view.Screen and drop layout dependency"
TAG: v0.4.15
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.

# Migración de UI a view.Screen y eliminación de dependencia webtyp/layout

## Ecosystem search

- `webtyp.com/view` (`v0.6.45`): define el contrato de pantalla declarativa `view.Screen`, `view.Presenter`, `view.WithDescription`, `view.WithTitle`, y el validador `Screen.Validate()`.
- `veltylabs/payment_manager`: actualmente exporta `ui.Browser(...) (platformd.UIModule, error)` importando `webtyp.com/layout/platformd`, forzando a este módulo a acoplarse con librerías de chasis visual y renderizado.

## Design gate

1. **Prior art**: `webtyp/view` v0.6.44 introdujo `view.Screen` como el contrato único para que los módulos declaren qué muestran como datos puros, sin importar renderizadores ni chasis.
2. **Novice-name test**: `ui.Screens(caller router.Caller) []view.Screen` devuelve las pantallas que ofrece este módulo al menú de la aplicación.
3. **Complexity ledger**: Elimina `webtyp.com/layout` (y sus dependencias indirectas) de `go.mod`.
4. **Where it belongs**: `ui.Screens` vive en `github.com/veltylabs/payment_manager/ui`. `NewView` vive en la raíz del paquete `paymentmanager`.
5. **What it deletes**: Borra `ui.Browser` y el require de `webtyp.com/layout` en `go.mod`.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go get webtyp.com/view@v0.6.45
go get webtyp.com/router@v0.4.8
```

## APIs used

| Symbol | Signature | Tag | File |
|---|---|---|---|
| `view.Screen` | `type Screen struct { ID string; Label lang.Text; Icon string; Permission model.Resource; Presenter Presenter; Lookups []Lookup; Help lang.Text; Children []Screen }` | `webtyp.com/view v0.6.45` | `screen.go:35` |
| `view.Presenter` | `type Presenter interface { ... }` | `webtyp.com/view v0.6.45` | `presenter.go:12` |
| `view.WithTitle` | `func WithTitle(title string) Option` | `webtyp.com/view v0.6.45` | `view.go:124` |
| `view.WithDescription` | `func WithDescription(d lang.Text) Option` | `webtyp.com/view v0.6.45` | `view.go:138` |
| `view.New` | `func New(l Lister, record model.Model, opts ...Option) Presenter` | `webtyp.com/view v0.6.45` | `view.go:147` |
| `router.Caller` | `type Caller interface { Call(op string, args Encodable, out Decodable, done func(error)); Dispatch(op string, args Encodable) }` | `webtyp.com/router v0.4.8` | `router.go` |

## Deletions

- `ui.Browser` en `ui/browser.go`. Criterio: `grep -rn "func.*Browser(" ui/` -> vacío.
- Todo import de `webtyp.com/layout` en el repositorio. Criterio: `grep -rn "webtyp.com/layout" .` -> vacío.

---

## Stage 1: Actualizar NewView en view.go y ui/paymentsview.go

En `view.go`:
Permitir que `NewView` reciba `caller router.Caller, opts ...view.Option`.
El tenant no se pasa por argumento fijo ya que `caller` lo inyecta a través de la sesión o headers.

Reemplazar `view.go`:

```go
package paymentmanager

import (
	"webtyp.com/lang"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/time"
	"webtyp.com/view"
)

// ListWindowSeconds es cuánto hacia atrás muestra la lista de pagos: 30 días.
const ListWindowSeconds = 30 * time.Day

const titlePayments = "Payments"

// Item proyecta un pago como fila: monto y medio; estado y referencia.
func (p *Payment) Item() view.Item {
	return view.Item{
		ID:          p.Id,
		Label:       FormatAmount(p.Amount, p.Currency) + " · " + lang.Translate(p.Method).String(),
		Description: lang.Translate(p.Status).String() + " · " + p.ReferenceModule + " " + p.ReferenceId,
	}
}

// NewView construye el Presenter de solo lectura de los pagos de los últimos ListWindowSeconds.
func NewView(caller router.Caller, opts ...view.Option) view.Presenter {
	allOpts := append([]view.Option{view.WithTitle(titlePayments)}, opts...)
	return view.New(&paymentLister{caller: caller}, &Payment{}, allOpts...)
}

type paymentLister struct {
	caller router.Caller
}

func (l *paymentLister) List(done func([]model.Model, error)) {
	now := time.NowUnix()
	out := &PaymentList{}
	l.caller.Call(ModelName+"."+OpListPayments,
		&ListPaymentsArgs{From: int64(now - ListWindowSeconds), To: int64(now + 1)}, out,
		func(err error) {
			if err != nil {
				done(nil, err)
				return
			}
			rows := make([]model.Model, len(*out))
			for i, p := range *out {
				rows[i] = p
			}
			done(rows, nil)
		})
}
```

En `ui/paymentsview.go`:
Actualizar la línea 71 para llamar `presenter: paymentmanager.NewView(caller)`.

En `tests/view_test.go`:
Actualizar las llamadas `pm.NewView(loopback.WithTenant(tenant, e.m), tenant)` a `pm.NewView(loopback.WithTenant(tenant, e.m))` y `pm.NewView(errCaller{}, tenant)` a `pm.NewView(errCaller{})`.

---

## Stage 2: Declarar ui.Screens y actualizar demo web/browser.go

En `ui/browser.go`:
Eliminar `Browser`.
Implementar `Screens(caller router.Caller) []view.Screen`.

Reescribir `ui/browser.go`:

```go
package ui

import (
	paymentmanager "github.com/veltylabs/payment_manager"
	"webtyp.com/lang"
	"webtyp.com/router"
	"webtyp.com/view"
)

const descPayments lang.Text = "Payment records, reconciliation and refunds."

// Screens declara las pantallas que este módulo ofrece al menú de la app.
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
```

En `web/browser.go`:
Reemplazar el uso de `platformd` con un render DOM mínimo sin `webtyp.com/layout`:

```go
//go:build browser

package main

import (
	"github.com/veltylabs/payment_manager/ui"
	. "webtyp.com/dom"
	"webtyp.com/html"
	"webtyp.com/router/loopback"
)

func main() {
	caller := loopback.New()
	screens := ui.Screens(caller)
	appNode := html.Div().Child(
		html.H1().Text(string(screens[0].Label)),
		html.P().Text("Demo"),
	)
	Append("body", appNode)
	select {}
}
```

---

## Stage 3: Actualizar pruebas unitarias

En `tests/ui_view_test.go`:
Crear o actualizar la prueba unitaria:

```go
//go:build browser

package tests

import (
	"testing"

	"github.com/veltylabs/payment_manager/ui"
	"webtyp.com/model"
	"webtyp.com/router"
)

type mockCaller struct{}

func (c *mockCaller) Call(route string, args model.Encodable, res model.Decodable, cb func(error)) {
	if cb != nil {
		cb(nil)
	}
}

func (c *mockCaller) Dispatch(route string, args model.Encodable) {}

var _ router.Caller = (*mockCaller)(nil)

func TestUI_Screens(t *testing.T) {
	fake := &mockCaller{}
	screens := ui.Screens(fake)
	if len(screens) != 1 {
		t.Fatalf("len(screens) = %d, want 1", len(screens))
	}
	s := screens[0]
	if err := s.Validate(); err != nil {
		t.Fatalf("screen.Validate() error: %v", err)
	}
	if s.ID != ui.ID {
		t.Errorf("ID = %q, want %q", s.ID, ui.ID)
	}
	if s.Presenter.Description() == "" {
		t.Error("Description() is empty")
	}
}
```

---

## Stage 4: Limpieza de dependencias en go.mod y verificación

Ejecutar:
```bash
go mod tidy
```

Verificar que:
1. `grep "webtyp.com/layout" go.mod` retorna vacío.
2. `grep "webtyp.com/layout" go.sum` retorna vacío.
3. `grep -rn "webtyp.com/layout" .` retorna vacío (excepto menciones en AGENTS.md o docs).

Ejecutar la suite de pruebas completa:
```bash
gotest
```

---

## Stage 5: Documentación y regla de idioma en veltylabs

1. En `AGENTS.md`, actualizar la sección de `ui/` indicando `Screens(caller router.Caller) []view.Screen`.
2. Ejecutar `langc sync .` y completar en `lang.json` cualquier clave faltante en español `(es)`:
   - Debe reportar 0 no traducidas (`es`).
   - 0 advertencias de reúso (`share ... — extract it as its own key and reuse it`).
   - 0 valores idénticos erróneos.

---

## Stages table

| Etapa | Archivos modificados | Qué hace |
|---|---|---|
| Stage 1 | `view.go`, `ui/paymentsview.go`, `tests/view_test.go` | Actualiza `NewView` con `opts ...view.Option` |
| Stage 2 | `ui/browser.go`, `web/browser.go` | Exporta `ui.Screens` y elimina imports de `layout` |
| Stage 3 | `tests/ui_view_test.go` | Valida `ui.Screens` |
| Stage 4 | `go.mod`, `go.sum` | `go mod tidy` y ejecución de `gotest` |
| Stage 5 | `AGENTS.md`, `lang.json` | Actualiza documentación en español y sincroniza `langc` |

## Delivery check

1. `grep -rn "webtyp.com/layout" --include="*.go" .` retorna vacío.
2. `gotest` pasa con 0 fallos de compilación, 0 fallos de tests y cobertura verificada.
3. `langc sync .` pasa sin errores y con 0 untranslated (es).
