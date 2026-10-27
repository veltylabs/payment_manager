# Payment Manager
<img src="docs/img/badges.svg">

Módulo de dominio para registrar pagos de mesón (efectivo, débito, crédito, transferencia) y pagos online (Webpay Plus, vía una pasarela inyectada). Maneja devoluciones y la conciliación con la pasarela. Diseño: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md); flujos y ramas probadas: [docs/diagrams/flows.md](docs/diagrams/flows.md).

## Inicio rápido

En la raíz del repo, abre la demo (pantalla Pagos y un paso de cobro de mesón, con la pasarela en memoria `sandbox`):

```bash
webtyp
```

## Operaciones

| Op | Descripción | Requiere |
|---|---|---|
| `get_charge` | Consulta el valor a pagar y su moneda | `payment_online`, `Read` |
| `create_counter_payment` | Registra un pago de mesón (genera un número de recibo secuencial) | `payment`, `Create` |
| `start_online_payment` | Inicia flujo webpay | `payment_online`, `Create` |
| `get_payment_status` | Estado actual de un pago | `payment_online`, `Read` |
| `get_payment` | Obtiene un pago (limpio de token) | `payment`, `Read` |
| `list_payments_by_reference` | Pagos de una reserva/referencia | `payment`, `Read` |
| `list_payments` | Lista pagos | `payment`, `Read` |
| `refund_payment` | Realiza una devolución parcial o total | `refund`, `Create` |
| `list_refunds` | Devoluciones de un pago | `refund`, `Read` |
| `start_terminal_payment` | Inicia cobro en máquina | `payment`, `Create` |
| `refresh_terminal_payment` | Refresca el estado en la máquina | `payment`, `Update` |
| `reconcile_payments` | Cierra pagos online y de máquina pendientes | `payment`, `Update` |

## Estados

| Estado | Descripción |
|---|---|
| `PENDING` | Pago online iniciado, esperando respuesta de pasarela |
| `PAID` | Pago completado y acreditado |
| `FAILED` | Pago rechazado por la pasarela |
| `CANCELLED` | El cliente abortó o canceló el flujo webpay, o cancelado en la máquina |
| `EXPIRED` | El pago webpay no fue completado a tiempo |
| `PARTIALLY_REFUNDED` | Se devolvió parte del monto, pero queda saldo |
| `REFUNDED` | Devolución total del monto |

## Cableado en una app

```go
gw, _ := transbank.Open(env.Get(transbank.EnvWebpay)) // github.com/veltylabs/transbank
pos := tuu.NewTerminal()
pm, err := paymentmanager.New(db, paymentmanager.Deps{
	Publisher: broker,
	Gateway:   gw,
	Terminal:  pos,
	Charges:   []paymentmanager.ChargeSource{{Module: "appointment_booking", Reader: bookingModule, Device: "POS-12345"}},
})
```

- El esquema lo crea `migrate.Migrate(conn, compiler)` desde el `cmd/migrate` de la app.
- La app corre `reconcile_payments` cada minuto.
- `ui.PaymentStep` es el paso de cobro que incrustan otras pantallas; `ui.Browser` es la pantalla Pagos; `ui.TakeReturn` lee la vuelta de la pasarela.

## Economic facts

Este módulo publica [hechos económicos](https://github.com/webtyp/money) (`money.Fact`) usando la dependencia `webtyp.com/money`. Cuando un pago es completado exitosamente o una devolución es registrada, se publican hechos `Charged` y `Refunded` (respectivamente) sobre el `events.Publisher` configurado en `Deps.Publisher`. El `Scope` corresponde al módulo que pidió el cobro (`ReferenceModule`). La contabilidad y seguimiento de caja quedan fuera de `payment_manager` y se realizan en otro lado suscribiéndose a estos hechos (`money.Subscribe`).

## Archivos clave

| Archivo | Qué contiene |
|---|---|
| `model.go` / `model_orm.go` | Modelos (`model_orm.go` lo genera `ormc`; no editar) |
| `fsm.go` | Máquina de estados del pago |
| `ports.go` | Puertos `Gateway` y `ChargeReader` |
| `module.go` / `repository.go` | Servicio y consultas |
| `ops.go` | Operaciones, recursos RBAC y códigos HTTP |
| `view.go` | Presenter de la lista de pagos |
| `amount.go` | `FormatAmount` |
| `sandbox/` | Pasarela en memoria para pruebas y demo |
| `migrate/` | Esquema (`payment`, `refund`) |
| `ui/` | Paso de pago, pantalla Pagos, vuelta de la pasarela |
| `seed/`, `web/` | Datos y demo |
