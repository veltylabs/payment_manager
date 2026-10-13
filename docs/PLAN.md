---
PLAN: "feat: payment_manager v0.1.0 — pagos de mesón y online, devoluciones, conciliación y pasarela inyectada"
TAG: v0.1.0
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 12145558634456377497
---

> Este plan se despacha con el flujo CodeJob. Ver skill: agents-workflow.

> **Pieza P** de
> [`mjosefa-cms/docs/RESERVA_Y_PAGOS_MASTER_PLAN.md`](https://github.com/veltylabs/mjosefa-cms/blob/main/docs/RESERVA_Y_PAGOS_MASTER_PLAN.md)
> (decisiones RP-2 a RP-13). **Requiere** `webtyp.com/dom` con `GetQuery`, `ClearQuery` y
> `NavigatePost`: **`webtyp.com/dom` v0.13.25** (pieza A2, ya publicada); usar esa versión o una posterior.
> Lee primero [`AGENTS.md`](../AGENTS.md): sus reglas aplican a todo este plan.

# Plan — `payment_manager`: pagos de mesón y online con devoluciones

## 0. Contexto (leer primero)

Repo nuevo (solo existe el esqueleto de `gonew`: `payment_manager.go` con un `PaymentManager`
vacío, que **se borra** en la etapa 0). Este módulo registra, confirma y devuelve pagos de
cualquier cosa que se cobre; el primer cliente es `appointment_booking` (cobro de una reserva).

Dos canales:

- **Mesón (`counter`)**: el cajero cobra en efectivo, débito, crédito o transferencia y lo
  registra. El pago nace pagado. Funciona sin conexión (la app lo reenvía después con el mismo
  id).
- **Online (`online`)**: un paciente paga con Webpay Plus de Transbank. Flujo:
  1. `start_online_payment` crea el pago `PENDING` y pide a la pasarela un token y una URL.
  2. El navegador sale de la app con un formulario POST (`token_ws`) a esa URL.
  3. Transbank devuelve el navegador a una ruta del servidor de la app (la monta el adaptador
     `veltylabs/transbank`, no este módulo). Esa ruta llama `Module.CompleteOnlinePayment`, que
     confirma con la pasarela y deja el pago `PAID` o `FAILED`/`CANCELLED`/`EXPIRED`.
  4. El servidor redirige a `/?payment=<id>&status=<estado>#<pantalla>`.
  5. Si el retorno se pierde, `reconcile_online_payments` cierra el pago consultando la pasarela.

Reglas de dinero y seguridad que no se negocian:

- **El monto nunca viene del cliente.** Lo resuelve `ChargeReader` del módulo dueño de la
  referencia (p. ej. `appointment_booking` devuelve el precio congelado de la reserva).
- **`int64` en unidades mínimas + moneda** (`CLP` sin decimales, `USD` con 2). Nunca `float`.
- **Devoluciones dentro del alcance**: en online la pasarela decide reversa (total, antes de 3 h)
  o anulación (hasta 90 días; parcial según el tipo de tarjeta); en mesón se registra la
  devolución y su medio.

Hechos de Webpay Plus (REST v1.2) que este módulo modela en forma genérica (los detalles HTTP
son del adaptador): token con vida de 5 minutos; formulario de pago con límite de 4 minutos
en producción; estado consultable 7 días; reversa total antes de 3 h desde la confirmación;
anulación hasta 90 días.

## Design gate

### 1. Prior art
- **Stripe PaymentIntent**: id, monto, moneda, estados `requires_payment_method → processing →
  succeeded/canceled`; reembolsos como objeto propio (`Refund`) contra el pago; clave de
  idempotencia en cada creación.
- **Transbank Webpay Plus REST**: crear (`buy_order`, `session_id`, `amount`, `return_url`) →
  redirigir con `token_ws` → confirmar (`PUT /transactions/{token}`) → consultar estado →
  reembolsar (`POST /transactions/{token}/refunds`, resultado `REVERSED` o `NULLIFIED`).
- **FHIR `PaymentNotice`/`PaymentReconciliation`**: el pago referencia lo que paga y la
  conciliación es un proceso separado de la captura.
- **Mesón**: Reservo y AgendaPro registran medio de pago y número de comprobante.

### 2. Prueba del nombre
- `Payment`, `Refund`: "pago", "devolución".
- `CreateCounterPayment(cmd)`: "registra un pago hecho en el mesón".
- `StartOnlinePayment(cmd)`: "empieza un pago online y dame a dónde mandar al cliente".
- `CompleteOnlinePayment(ret)`: "termina el pago online con lo que devolvió la pasarela".
- `RefundPayment(cmd)`: "devuelve este monto de este pago".
- `ReconcileOnlinePayments(tenant, olderThan)`: "cierra los pagos online que quedaron
  pendientes".
- `Gateway` (`Start`, `Commit`, `Status`, `Refund`): "la pasarela".
- `ChargeReader.Charge(tenant, referenceId)`: "¿cuánto se cobra por esto?".
- `FormatAmount(amount, currency)`: "muéstrame el monto como lo lee una persona" (`$25.000`).

### 3. Libro de complejidad
Repo nuevo: 2 entidades persistidas (`Payment`, `Refund`), 1 FSM, 2 interfaces de puerto
(`Gateway`, `ChargeReader`), 9 ops, 1 subpaquete `sandbox/` (pasarela en memoria),
`migrate/`, `ui/` (paso de pago + pantalla de pagos), `seed/`, `web/`.

### 4. Dónde vive
Módulo de dominio propio (RP-2): nadie más debe saber de medios de pago, reversas o
conciliación. La pasarela concreta vive en `veltylabs/transbank` (RP-3).

### 5. Qué borra
`payment_manager.go` del esqueleto. En `appointment_booking` (pieza B) evita que el pago viva
dentro de la reserva.

## 1. Reglas del repositorio (resumen inline de `AGENTS.md`; aplicar sin excepción)

- Paquete raíz: solo `webtyp.com/{model,router,view,events,orm,storage,ddl,input,fmt,time}`.
  **Prohibido** `net/http`, `strings`, `strconv`, `errors`, `fmt` (stdlib), `encoding/json`,
  `webtyp.com/unixid`, cualquier driver de base de datos, cualquier renderizador.
- **Sin `map`** en ningún archivo, pruebas incluidas. Sin `reflect`.
- Modelos como `model.Definition` en `model.go`; `model_orm.go` lo genera
  `go install webtyp.com/ormc/cmd/ormc@latest && ormc` en la raíz. **Nunca editarlo a mano.**
- Cada escritura llama `Validate(action)` antes de `db.Create`/`db.Update`.
- Cada `UPDATE` lleva `tenant_id` en la condición; concurrencia optimista con `revision`
  (`WHERE revision = N`, desacuerdo ⇒ `ErrConflict`).
- Op: decode (400) → `Validate` (400) → servicio → encode. Estados: 400 validación/precondición,
  404 no encontrado, 409 conflicto, 500 interno.
- Literales de enumeración solo en constantes exportadas.
- **Todas** las pruebas en `tests/` (paquete `tests`, solo API exportada; ningún `_test.go`
  fuera de `tests/`, tampoco en `migrate/`, `sandbox/` ni `ui/`) con `orm.New(mem.New())` y
  `router/mock`; ejecutar con `gotest`. No exportar nada solo para una prueba.
- `ui/` puede importar `webtyp.com/{dom,html,components/*,layout/*,widget}`; `css.go` y `svg.go`
  con `//go:build !wasm`; sin clases escritas a mano (`widget.Name`/`Part`). Textos de UI en
  inglés como `lang.Text`, traducidos en `Render`, con su clave en `lang.json`.
- Documentación y comentarios en español.
- Implementación de referencia de forma y estilo:
  <https://github.com/veltylabs/appointment_booking> (`model.go`, `fsm.go`, `service.go`,
  `ops.go`, `repository.go`, `migrate/`, `ui/`, `seed/`, `web/`).

## 1.1 Método: diagramas → pruebas en rojo → código (TDD), 100% de cobertura

Es dinero: ninguna línea de producción se escribe antes que la prueba que la exige.

- **Fuente de verdad de los flujos:** [`docs/diagrams/flows.md`](diagrams/flows.md) (ya
  escrito, no se modifica sin actualizar este plan). Cada rombo tiene un id (`C1`, `S6`, `K2`, …);
  cada **rama** (`C1-sí`, `C1-no`) tiene en §12 la prueba que la cubre, con su nombre exacto.
- **Orden obligatorio en cada etapa 1 a 9:**
  1. Escribir las pruebas de §12 que corresponden a esa etapa (compilan contra firmas vacías:
     crear la firma con `panic("sin implementar")` si hace falta para compilar).
  2. Correr `gotest` y **confirmar que fallan**. Commit `test(<etapa>): pruebas en rojo`.
  3. Implementar lo mínimo para que pasen. Commit `feat(<etapa>): …`.
  4. `gotest` en verde antes de pasar a la etapa siguiente.
  El historial del PR debe mostrar cada commit en rojo antes de su commit verde.
- **Cobertura 100%** de los paquetes `paymentmanager` (raíz, incluido el `model_orm.go`
  generado), `sandbox` y `migrate`, medida así (y además el badge de `gotest`):

  ```bash
  go test -coverpkg=github.com/veltylabs/payment_manager,github.com/veltylabs/payment_manager/sandbox,github.com/veltylabs/payment_manager/migrate \
    -coverprofile=/tmp/cover.out ./tests/... && go tool cover -func=/tmp/cover.out | tail -1
  # debe imprimir: total: (statements) 100.0%
  ```

  El código generado se cubre con pruebas que **verifican** algo: ida y vuelta
  encode → decode campo por campo y `Validate` aceptando un válido y rechazando cada
  restricción declarada. Una prueba que llama y descarta el resultado no cuenta (AGENTS.md).
  Si una línea no se puede alcanzar desde la API pública, es código muerto: se borra, no se
  exporta nada para alcanzarla.
- **Errores de base de datos** (lecturas y escrituras que fallan): se cubren con un
  `storage.Conn` falso declarado en `tests/setup_test.go` que envuelve `storage/mem` y falla a
  pedido en `Query`/`Exec` (por nombre de tabla y operación). `orm.New(eseConnFalso)` se pasa a
  `New`. Cada rama `if err != nil` de `module.go` y `repository.go` tiene una fila
  `TestStorageErrors_*` en §12.
- `ui/` y `web/` corren en wasm, donde no se mide cobertura: cada rama `U*` de
  [flows.md §10](diagrams/flows.md) tiene su prueba wasm (§12) y eso es el criterio.

## 2. Etapa 0 — esqueleto

- **Prerrequisito:** `go install webtyp.com/devflow/cmd/gotest@latest` y
  `go install webtyp.com/ormc/cmd/ormc@latest`.
- Borrar `payment_manager.go`.
- `go.mod`: módulo `github.com/veltylabs/payment_manager`; dependencias por `go mod tidy`.
- Crear `lang.json` en la raíz (`{"languages": ["es"], "keys": {}}`) y llenarlo en cada etapa
  que agregue textos.

## 3. Etapa 1 — `model.go` (+ `model_orm.go` regenerado)

Constantes (exportadas, únicas dueñas de sus literales):

```go
const ModelName = "payment_manager"

const (
	ChannelCounter = "counter"
	ChannelOnline  = "online"
)

const (
	MethodCash     = "cash"
	MethodDebit    = "debit"
	MethodCredit   = "credit"
	MethodTransfer = "transfer"
	MethodWebpay   = "webpay"
)

const (
	CurrencyCLP = "CLP"
	CurrencyUSD = "USD"
)

const (
	StatusPending           = "PENDING"
	StatusPaid              = "PAID"
	StatusFailed            = "FAILED"
	StatusCancelled         = "CANCELLED" // el cliente abandonó el pago online
	StatusExpired           = "EXPIRED"   // venció sin respuesta de la pasarela
	StatusPartiallyRefunded = "PARTIALLY_REFUNDED"
	StatusRefunded          = "REFUNDED"
)

const (
	RefundKindReversed  = "REVERSED"  // pasarela: reversa total
	RefundKindNullified = "NULLIFIED" // pasarela: anulación total o parcial
	RefundKindManual    = "MANUAL"    // mesón: devolución registrada por el cajero
)

// Tiempos (segundos).
const (
	OnlinePendingTimeout = 600        // un pago online PENDING más viejo que esto se concilia
	GatewayStatusWindow  = 7 * 86400  // la pasarela guarda el estado 7 días
)
```

Definiciones (`model.Definition`), con widgets solo donde un usuario edita en un formulario:

**`payment`** — columnas en este orden:
`id` (Text, PK, NotNull, `Permitted{Letters, Numbers, Extra: '.', '-', '_', Min 1, Max 26}` —
cabe en la orden de compra de Webpay), `tenant_id` (NotNull), `reference_module` (NotNull, max
60), `reference_id` (NotNull, max 64), `channel` (NotNull), `method` (NotNull), `amount` (Int,
NotNull), `currency` (NotNull, 3 letras), `status` (NotNull), `refunded_amount` (Int),
`provider_token` (Text, OmitEmpty), `authorization_code` (OmitEmpty), `payment_type_code`
(OmitEmpty), `installments` (Int, OmitEmpty), `card_last4` (OmitEmpty), `receipt_number`
(OmitEmpty, max 40), `return_to` (OmitEmpty, `Permitted{Letters, Numbers, Extra: '_', '-', Max 60}`),
`created_by` (OmitEmpty), `created_at` (Int), `paid_at` (Int, OmitEmpty), `updated_at` (Int),
`revision` (Int).

**`refund`**: `id` (PK, mismas reglas que `payment.id`), `tenant_id`, `payment_id` (Ref a
`payment`, `DB.RefColumn: "id"`), `amount` (Int, NotNull), `kind` (NotNull), `method`
(OmitEmpty; en mesón el medio de la devolución), `reason` (`input.Textarea()`, NotNull, max 255),
`authorization_code` (OmitEmpty), `created_by` (OmitEmpty), `created_at` (Int).

**Args y resultados** (sin widgets salvo indicación; todos con `tenant_id` NotNull):

| Modelo | Campos |
|---|---|
| `create_counter_payment_args` | `tenant_id`, `id`, `reference_module`, `reference_id`, `method` (radio cerrado: cash/debit/credit/transfer), `receipt_number` |
| `start_online_payment_args` | `tenant_id`, `id`, `reference_module`, `reference_id`, `return_to` |
| `start_online_payment_result` | `payment_id`, `redirect_url`, `token` |
| `get_payment_args` | `tenant_id`, `id` |
| `payment_status_result` | `id`, `status` |
| `get_charge_args` | `tenant_id`, `reference_module`, `reference_id` |
| `charge_result` | `amount`, `currency`, `payable` (Bool) |
| `list_payments_by_reference_args` | `tenant_id`, `reference_module`, `reference_id` |
| `list_payments_args` | `tenant_id`, `from`, `to` (Int, segundos; filtra `created_at`) |
| `refund_payment_args` | `tenant_id`, `id` (de la devolución), `payment_id`, `amount`, `reason`, `method` |
| `list_refunds_args` | `tenant_id`, `payment_id` |
| `reconcile_online_payments_args` | `tenant_id` |
| `reconcile_result` | `closed` (Int) |

Regenerar `model_orm.go` con `ormc`.

## 4. Etapa 2 — `fsm.go`

Tabla en slice (como `appointment_booking/fsm.go`), con eventos exportados:
`EventPay`, `EventFail`, `EventCancel`, `EventExpire`, `EventRefundPartial`, `EventRefundTotal`.

| Desde | Evento | Hacia |
|---|---|---|
| PENDING | PAY | PAID |
| PENDING | FAIL | FAILED |
| PENDING | CANCEL | CANCELLED |
| PENDING | EXPIRE | EXPIRED |
| PAID | REFUND_PARTIAL | PARTIALLY_REFUNDED |
| PAID | REFUND_TOTAL | REFUNDED |
| PARTIALLY_REFUNDED | REFUND_PARTIAL | PARTIALLY_REFUNDED |
| PARTIALLY_REFUNDED | REFUND_TOTAL | REFUNDED |

`func Transition(current, event string) (string, error)`; transición inválida ⇒
`ErrInvalidTransition`.

## 5. Etapa 3 — `ports.go`, `errors.go`, `amount.go`

```go
// Gateway es la pasarela de pago online. La implementación concreta (Transbank)
// vive fuera de este módulo y la inyecta la app. Sus llamadas son síncronas: solo
// se usa en el servidor.
type Gateway interface {
	Start(req GatewayStart) (GatewayStarted, error)
	Commit(token string) (GatewayResult, error)
	Status(token string) (GatewayResult, error)
	Refund(token string, amount int64) (GatewayRefund, error)
}

type GatewayStart struct {
	BuyOrder  string // = Payment.Id
	SessionId string // = Payment.Id
	Amount    int64
	Currency  string
	ReturnURL string // = Deps.ReturnURL
}

type GatewayStarted struct {
	Token       string
	RedirectURL string
}

// Resultado de Commit/Status, ya traducido por el adaptador.
const (
	OutcomePending    = "pending"    // la pasarela aún no lo cierra
	OutcomeAuthorized = "authorized"
	OutcomeRejected   = "rejected"
)

type GatewayResult struct {
	Outcome           string
	Amount            int64
	AuthorizationCode string
	PaymentTypeCode   string
	Installments      int64
	CardLast4         string
}

type GatewayRefund struct {
	Kind              string // RefundKindReversed | RefundKindNullified
	AuthorizationCode string
	Amount            int64 // monto efectivamente devuelto
}

// ChargeReader responde cuánto se cobra por una referencia de SU módulo. Lo
// implementa el módulo dueño (appointment_booking.Module) sin importar este paquete.
type ChargeReader interface {
	Charge(tenantId, referenceId string) (amount int64, currency string, payable bool, err error)
}

// ChargeSource asocia un módulo de referencia con su lector. Slice en Deps, no map.
type ChargeSource struct {
	Module string // = reference_module, p. ej. "appointment_booking"
	Reader ChargeReader
}

// OnlineReturn es lo que la pasarela devolvió al navegador, ya interpretado por
// el adaptador. Exactamente uno de los tres casos:
//   - Token != "" y AbortToken == "": volvió normal (aprobado o rechazado).
//   - AbortToken != "": el cliente abandonó (o error con "volver al sitio").
//   - solo BuyOrder != "": venció el formulario de pago.
type OnlineReturn struct {
	Token      string
	AbortToken string
	BuyOrder   string
}
```

`errors.go` — `type domainError string` y estos centinelas, con su estado HTTP:

| Error | Texto exacto | HTTP |
|---|---|---|
| `ErrNotFound` | `payment not found` | 404 |
| `ErrRefundNotFound` | `refund not found` | 404 |
| `ErrMissingArgs` | `missing required arguments` | 400 |
| `ErrUnknownReference` | `no charge reader for this reference module` | 400 |
| `ErrNotPayable` | `the reference has nothing to pay` | 400 |
| `ErrUnsupportedCurrency` | `unsupported currency` | 400 |
| `ErrInvalidMethod` | `invalid payment method for this channel` | 400 |
| `ErrInvalidTransition` | `invalid payment status transition` | 400 |
| `ErrRefundExceeds` | `refund amount exceeds what is left to refund` | 400 |
| `ErrIdTaken` | `id already used by another operation` | 409 |
| `ErrAlreadyPaid` | `the reference is already paid` | 409 |
| `ErrNotPaid` | `payment is not paid` | 409 |
| `ErrConflict` | `payment changed concurrently` | 409 |
| `ErrGatewayUnavailable` | `online payments are not configured` | 500 |
| `ErrGatewayFailed` | `payment gateway call failed` | 500 |
| `ErrAmountMismatch` | `gateway amount differs from the payment amount` | 500 |

`amount.go`:

```go
// FormatAmount muestra un monto en unidades mínimas como lo lee una persona en
// Chile: CLP → "$25.000"; USD → "US$12,50". Moneda desconocida → el número y el código.
func FormatAmount(amount int64, currency string) string
```

Implementación sin stdlib: agrupar miles con `.` recorriendo los dígitos; `USD` divide por 100
y usa `,` para los decimales.

## 6. Etapa 4 — `module.go` (servicio) y `repository.go`

```go
type Deps struct {
	Publisher events.Publisher // opcional — nil desactiva
	Gateway   Gateway          // opcional — nil: las ops online responden ErrGatewayUnavailable
	ReturnURL string           // requerido si Gateway != nil: URL absoluta de la ruta de retorno
	Charges   []ChargeSource   // requerido: al menos uno
}

func New(db *orm.DB, deps Deps) (*Module, error)
```

`New` falla con `fmt.Err("payment_manager: Deps.Charges is required")` si no hay lectores y
con `fmt.Err("payment_manager: Deps.ReturnURL is required with a Gateway")` si hay pasarela sin
URL.

Eventos (constantes exportadas; payload siempre `*Payment` o `*Refund`):
`EventPaymentPaid = "payment.paid"`, `EventPaymentFailed = "payment.failed"`,
`EventPaymentRefunded = "payment.refunded"` (payload `*Refund`).

Métodos y reglas exactas:

1. **`Charge(tenantId, referenceModule, referenceId string) (amount int64, currency string, payable bool, err error)`**:
   busca el `ChargeSource` por `Module` (escaneo lineal) → `ErrUnknownReference` si no está;
   delega; moneda fuera de {CLP, USD} ⇒ `ErrUnsupportedCurrency`.
2. **`CreateCounterPayment(cmd CreateCounterPaymentCmd) (Payment, error)`** (`cmd` lleva
   `TenantId, Id, ReferenceModule, ReferenceId, Method, ReceiptNumber, ActorId`):
   - Idempotencia: si existe un pago con ese id y mismo tenant y misma referencia ⇒ devolverlo;
     si existe con otro tenant o referencia ⇒ `ErrIdTaken`.
   - `Method` ∈ {cash, debit, credit, transfer}, si no ⇒ `ErrInvalidMethod`.
   - Si la referencia ya tiene un pago `PAID` o `PARTIALLY_REFUNDED` ⇒ `ErrAlreadyPaid`.
   - `Charge(...)`; `payable == false` ⇒ `ErrNotPayable`.
   - Crea con `Channel = counter`, `Status = PAID`, `PaidAt = CreatedAt = now`, `Revision = 1`.
   - Publica `EventPaymentPaid`.
3. **`StartOnlinePayment(cmd StartOnlinePaymentCmd) (StartOnlinePaymentResult, error)`**
   (`TenantId, Id, ReferenceModule, ReferenceId, ReturnTo`):
   - `Gateway == nil` ⇒ `ErrGatewayUnavailable`.
   - Id ya usado ⇒ `ErrIdTaken`, siempre. Cada intento de pago online usa un id nuevo (la UI
     lo genera en cada clic): el token de la pasarela vive 5 minutos, así que reutilizar un
     intento viejo no sirve. Los intentos abandonados los cierra la conciliación.
   - `ErrAlreadyPaid` como en mesón; `Charge` y `ErrNotPayable` igual.
   - Crea `PENDING`, `Channel = online`, `Method = webpay`.
   - `Gateway.Start(GatewayStart{BuyOrder: id, SessionId: id, Amount, Currency, ReturnURL})`;
     error ⇒ pasar el pago a `FAILED` y devolver `ErrGatewayFailed`.
   - Guarda `ProviderToken`; devuelve `{PaymentId, RedirectURL, Token}`.
4. **`CompleteOnlinePayment(ret OnlineReturn) (Payment, error)`** (lo llama el adaptador desde
   la ruta de retorno; no es op):
   - Token normal: buscar por `provider_token`. Si ya no está `PENDING` ⇒ devolverlo tal cual
     (idempotente: Transbank puede repetir el retorno). `Gateway.Commit(token)`:
     `authorized` con `Amount == payment.Amount` ⇒ `PAY` (guarda código, tipo, cuotas, últimos
     4, `PaidAt`), publica `EventPaymentPaid`; `authorized` con monto distinto ⇒
     `Gateway.Refund(token, result.Amount)` y `FAIL`, devolver `ErrAmountMismatch` junto al pago;
     `rejected` ⇒ `FAIL`, publica `EventPaymentFailed`; error de la pasarela ⇒ dejar `PENDING`
     (lo cierra la conciliación) y devolver `ErrGatewayFailed`.
   - `AbortToken`: buscar por `provider_token = AbortToken` ⇒ `CANCEL`.
   - Solo `BuyOrder`: buscar por id ⇒ `EXPIRE`.
   - Pago no encontrado ⇒ `ErrNotFound`.
5. **`RefundPayment(cmd RefundPaymentCmd) (Refund, error)`** (`TenantId, Id, PaymentId, Amount,
   Reason, Method, ActorId`):
   - Idempotencia por id de devolución (igual que en pagos).
   - El pago debe estar `PAID` o `PARTIALLY_REFUNDED` (si no ⇒ `ErrInvalidTransition`).
   - `0 < Amount <= payment.Amount - payment.RefundedAmount`, si no ⇒ `ErrRefundExceeds`.
   - Online: `Gateway.Refund(token, Amount)` (error ⇒ `ErrGatewayFailed`, nada se escribe);
     `Kind` y código del resultado; si la pasarela devolvió otro monto, se registra el devuelto.
   - Mesón: `Kind = MANUAL`; `Method` ∈ {cash, debit, credit, transfer}.
   - En una transacción: crear `Refund`; sumar a `RefundedAmount`; evento
     `REFUND_TOTAL` si queda en 0, si no `REFUND_PARTIAL`; `revision++` con
     `WHERE revision = N`.
   - Publica `EventPaymentRefunded`.
6. **`ReconcileOnlinePayments(tenantId string, now int64) (int, error)`**: para cada pago
   `online` `PENDING` con `CreatedAt < now - OnlinePendingTimeout`:
   - Si `CreatedAt < now - GatewayStatusWindow` ⇒ `EXPIRE`.
   - Si no, `Gateway.Status(token)`: `authorized` ⇒ igual que el commit autorizado del punto 4
     (incluye el control de monto); `rejected` ⇒ `FAIL`; `pending` ⇒ `EXPIRE` (el formulario
     ya venció); error ⇒ saltarlo y seguir.
   - Devuelve cuántos cerró.
7. **`PaidAmount(tenantId, paymentId, referenceModule, referenceId string) (amount int64, currency string, err error)`**:
   lo usa `appointment_booking` (su interfaz `PaymentReader`, satisfecha estructuralmente:
   solo tipos primitivos en la firma) para verificar un pago antes de confirmar una reserva.
   Devuelve el monto **neto** (`Amount - RefundedAmount`) solo si el pago existe en ese tenant,
   es de esa referencia exacta y está `PAID` o `PARTIALLY_REFUNDED`. Si no:
   `ErrNotFound` (no existe, otro tenant u otra referencia) o `ErrNotPaid` = `payment is not paid`
   (agregar a la tabla de errores, HTTP 409).
8. Lecturas: `GetPayment`, `ListPaymentsByReference`, `ListPayments(tenantId, from, to)`,
   `ListRefunds(tenantId, paymentId)`; todas filtran por tenant.

`repository.go`: las consultas, como en `appointment_booking/repository.go`.

## 7. Etapa 5 — `ops.go`

```go
const (
	OpGetCharge               = "get_charge"
	OpCreateCounterPayment    = "create_counter_payment"
	OpStartOnlinePayment      = "start_online_payment"
	OpGetPaymentStatus        = "get_payment_status"
	OpGetPayment              = "get_payment"
	OpListPaymentsByReference = "list_payments_by_reference"
	OpListPayments            = "list_payments"
	OpRefundPayment           = "refund_payment"
	OpListRefunds             = "list_refunds"
	OpReconcileOnlinePayments = "reconcile_online_payments"
)

// Recursos: la app concede ResourcePaymentOnline a la identidad anónima para la
// reserva online; el resto solo a personal.
const (
	ResourcePayment       = "payment"
	ResourcePaymentOnline = "payment_online"
	ResourceRefund        = "refund"
)
```

| Op | Requires | Args → respuesta |
|---|---|---|
| `get_charge` | `payment_online`, Read | `GetChargeArgs` → `ChargeResult` |
| `create_counter_payment` | `payment`, Create | → `Payment` |
| `start_online_payment` | `payment_online`, Create | → `StartOnlinePaymentResult` |
| `get_payment_status` | `payment_online`, Read | `GetPaymentArgs` → `PaymentStatusResult` (solo id y estado) |
| `get_payment` | `payment`, Read | → `Payment` sin `provider_token` (vaciarlo antes de codificar) |
| `list_payments_by_reference` | `payment`, Read | → `PaymentList` (sin token) |
| `list_payments` | `payment`, Read | → `PaymentList` (sin token) |
| `refund_payment` | `refund`, Create | → `Refund` |
| `list_refunds` | `refund`, Read | → `RefundList` |
| `reconcile_online_payments` | `payment`, Update | → `ReconcileResult` (usa `time.Now()` en segundos) |

`writeError` mapea con la tabla de la etapa 3; `orm.IsNotFound` ⇒ 404; el resto ⇒ 500.
`var _ router.OperationModule = (*Module)(nil)`.

## 8. Etapa 6 — `migrate/`

`migrate/migrate.go`: `func Migrate(conn ddl.Execer, c ddl.Compiler) error` que hace
`ddl.New(conn, c).Sync(&paymentmanager.Payment{}, &paymentmanager.Refund{})`. Su prueba va en
`tests/migrate_test.go` (todas las pruebas viven en `tests/`, sin excepción).

## 9. Etapa 7 — `sandbox/` (pasarela en memoria)

`sandbox/sandbox.go` (paquete `sandbox`, sin build tag, solo `webtyp.com/fmt` y el paquete
raíz): `type Gateway struct{ … }` que implementa `paymentmanager.Gateway` en memoria.

- `New(baseURL string) *Gateway`; `Start` devuelve token `sbx-<BuyOrder>` y
  `RedirectURL = baseURL`.
- Comportamiento controlable por la prueba o la demo con métodos exportados:
  `Approve(token)`, `Reject(token)`, `FailNext(err error)` (la próxima llamada falla),
  `SetCommitAmount(token, amount)` (para probar el monto distinto).
- `Refund`: devuelve `REVERSED` si es total y es la primera devolución, si no `NULLIFIED`.
- Sin `map`: slice de transacciones con escaneo lineal.

## 10. Etapa 8 — `ui/`

Requiere `dom` con `GetQuery`, `ClearQuery`, `NavigatePost` (pieza A2).

### 10.1 `ui/paymentstep.go` — el paso de pago que otras pantallas incrustan

```go
// PaymentStep cobra una referencia. En mesón registra el pago; en online lleva
// al cliente a la pasarela. Lo incrusta, por ejemplo, el paso 6 de la reserva.
type PaymentStep struct {
	dom.Element
	Caller          router.Caller
	IDs             model.IDGenerator
	TenantID        string
	Channel         string // paymentmanager.ChannelCounter | ChannelOnline
	ReferenceModule string
	ReferenceID     string
	ReturnTo        string             // online: id de la pantalla a la que se vuelve (hash)
	OnPaid          func(paymentID string) // mesón: tras registrar el pago
}
```

- Al iniciarse llama `get_charge` y muestra el monto con `FormatAmount`. `payable == false`:
  muestra "Nothing to pay" y no ofrece cobrar.
- **Mesón**: `segmentedcontrol` con Cash / Debit / Credit / Transfer, campo opcional
  "Receipt number", botón "Register payment" ⇒ `create_counter_payment` con id nuevo de `IDs`
  ⇒ `OnPaid(id)`. Error ⇒ mensaje visible con el texto del error (nunca tragarlo).
- **Online**: botón "Pay with Webpay" ⇒ `start_online_payment` (id nuevo; `ReturnTo`) ⇒
  `dom.NavigatePost(redirect_url, []fmt.KeyValue{{Key: "token_ws", Value: token}})`.
  Mientras espera, el botón queda deshabilitado (`widget.Busy`). Error ⇒ mensaje visible.

### 10.2 `ui/return.go` — leer la vuelta de la pasarela

```go
// TakeReturn lee y borra de la URL el resultado que dejó la ruta de retorno
// (?payment=<id>&status=<estado>). ok es false si la página no viene de un pago.
func TakeReturn() (paymentID, status string, ok bool)
```

Usa `dom.GetQuery("payment")`, `dom.GetQuery("status")` y `dom.ClearQuery()`. Las constantes
de los nombres de parámetro son exportadas en el paquete raíz y las usa también el adaptador:
`QueryPayment = "payment"`, `QueryStatus = "status"`.

### 10.3 `ui/` — pantalla "Pagos" (`Browser`)

En el paquete raíz, `view.go`: `func (p *Payment) Item() view.Item` (ver abajo) y
`func NewView(caller router.Caller, tenantID string) view.Presenter`, solo lectura, con un
`Lister` que llama `list_payments` (patrón `appointment_booking/view.go` `NewView`).

`const ID = "payment_manager"`, `const Label = "Payments"`, `Browser(caller, ids, tenantID)
(platformd.UIModule, error)`, como `appointment_booking/ui`. Contenido: un `crudview` de solo
lectura sobre `list_payments` de los últimos 30 días (`from = hoy - 30 días`, `to = ahora`),
con el buscador de texto por defecto de `crudview`. Cada fila (`Payment.Item()`): `Label` =
monto con `FormatAmount` + medio traducido; `Description` = estado traducido + referencia; el detalle del pago lista sus devoluciones (`list_refunds`) y ofrece
"Refund" ⇒ `modaldialog` con monto (por defecto lo que queda), motivo y, si es de mesón, medio
⇒ `refund_payment`. Botón "Reconcile online payments" ⇒ `reconcile_online_payments`.

### 10.4 Textos (inglés en código, español en `lang.json`)

`Nothing to pay` → `Nada que pagar`; `Cash` → `Efectivo`; `Debit` → `Débito`; `Credit` →
`Crédito`; `Transfer` → `Transferencia`; `Receipt number` → `N° de comprobante`;
`Register payment` → `Registrar pago`; `Pay with Webpay` → `Pagar con Webpay`;
`Payments` → `Pagos`; `Refund` → `Devolver`; `Reason` → `Motivo`; `Amount` → `Monto`;
`Reconcile online payments` → `Conciliar pagos online`; y cada estado y medio por su constante
(`PAID` → `Pagado`, `PENDING` → `Pendiente`, `FAILED` → `Rechazado`, `CANCELLED` →
`Abandonado`, `EXPIRED` → `Vencido`, `PARTIALLY_REFUNDED` → `Devuelto en parte`,
`REFUNDED` → `Devuelto`).

## 11. Etapa 9 — `seed/` y `web/`

- `seed/seed.go`: `Load(m *paymentmanager.Module, tenantID string, refs []Ref) (Data, error)`
  donde `Ref{Module, Id string}`; crea un pago de mesón pagado y uno con devolución parcial.
- `web/client.go` (`//go:build wasm`): demo con `storage/mem`, `router/loopback`,
  `events/mock`, `unixid`, `sandbox.New("/")` como pasarela y un `ChargeReader` de demostración
  declarado en el mismo archivo (`demoCharges`, devuelve `25000 CLP` para cualquier referencia).
  Muestra la pantalla "Pagos" y, en una segunda entrada, un `PaymentStep` de mesón.

## 12. Pruebas — trazabilidad diagrama → prueba

Cada fila se escribe **en rojo dentro de la etapa indicada** (§1.1). Nombres exactos. Archivos
chicos por tema en `tests/` (paquete `tests`). Pasarela: `sandbox`. Base: `orm.New(mem.New())`.
Eventos: un `events.Publisher` falso que registra lo publicado (declarado en
`tests/setup_test.go`). `ChargeReader`: falso en `tests/setup_test.go`, configurable por prueba.

| Rama | Prueba | Archivo | Etapa |
|---|---|---|---|
| FSM, cada fila y una inválida | `TestTransition_Table`, `TestTransition_Invalid` | `fsm_test.go` | 2 |
| `H1-no` | `TestCharge_UnknownModule` | `charge_test.go` | 4 |
| `H2-sí` | `TestCharge_ReaderError` | `charge_test.go` | 4 |
| `H3-no` | `TestCharge_UnsupportedCurrency` | `charge_test.go` | 4 |
| `H3-sí` | `TestCharge_OK` | `charge_test.go` | 4 |
| `C1-sí` | `TestCounter_MissingArgs` | `counter_test.go` | 4 |
| `C2-sí/C3-sí` | `TestCounter_RetryReturnsStoredWithoutEvent` | `counter_test.go` | 4 |
| `C3-no` | `TestCounter_IdTakenByOtherReference` | `counter_test.go` | 4 |
| `C4-no` | `TestCounter_WebpayMethodRejected` | `counter_test.go` | 4 |
| `C5-sí` | `TestCounter_AlreadyPaid` | `counter_test.go` | 4 |
| `C6-no` | `TestCounter_NotPayable` | `counter_test.go` | 4 |
| `C6-sí` | `TestCounter_CreatesPaidAndPublishes` | `counter_test.go` | 4 |
| `S1-no` | `TestStartOnline_NoGateway` | `online_start_test.go` | 4 |
| `S2-sí` | `TestStartOnline_MissingArgs` | `online_start_test.go` | 4 |
| `S3-sí` | `TestStartOnline_IdTaken` | `online_start_test.go` | 4 |
| `S4-sí` | `TestStartOnline_AlreadyPaid` | `online_start_test.go` | 4 |
| `S5-no` | `TestStartOnline_NotPayable` | `online_start_test.go` | 4 |
| `S6-sí` | `TestStartOnline_GatewayErrorMarksFailed` | `online_start_test.go` | 4 |
| `S6-no` | `TestStartOnline_PendingWithToken` | `online_start_test.go` | 4 |
| `R1-nada` | `TestComplete_EmptyReturn` | `online_complete_test.go` | 4 |
| `R2-no`, `R4-no`, `R6-no` | `TestComplete_UnknownAbortToken`, `TestComplete_UnknownBuyOrder`, `TestComplete_UnknownToken` | `online_complete_test.go` | 4 |
| `R3-no`, `R5-no`, `R7-no` | `TestComplete_AbortOnSettled`, `TestComplete_TimeoutOnSettled`, `TestComplete_RepeatedReturnIsIdempotent` | `online_complete_test.go` | 4 |
| `R3-sí` | `TestComplete_AbortCancels` | `online_complete_test.go` | 4 |
| `R5-sí` | `TestComplete_TimeoutExpires` | `online_complete_test.go` | 4 |
| `R8-sí` | `TestComplete_CommitErrorKeepsPending` | `online_complete_test.go` | 4 |
| `K1-authorized/K2-sí` | `TestComplete_ApprovedPaysAndPublishes` (guarda código, tipo, cuotas, últimos 4) | `online_complete_test.go` | 4 |
| `K2-no` | `TestComplete_AmountMismatchRefundsAndFails` | `online_complete_test.go` | 4 |
| `K1-rejected` | `TestComplete_RejectedFailsAndPublishes` | `online_complete_test.go` | 4 |
| `K1-pending/K3-no` | `TestComplete_PendingOutcomeKeepsPending` | `online_complete_test.go` | 4 |
| `F1-sí` | `TestRefund_MissingArgs` | `refund_test.go` | 4 |
| `F2-sí/F3-sí` | `TestRefund_RetryReturnsStored` | `refund_test.go` | 4 |
| `F3-no` | `TestRefund_IdTaken` | `refund_test.go` | 4 |
| `F4-no` | `TestRefund_OtherTenantNotFound` | `refund_test.go` | 4 |
| `F5-no` | `TestRefund_PendingPaymentRejected` | `refund_test.go` | 4 |
| `F6-no` | `TestRefund_ExceedsBalance`, `TestRefund_ZeroAmount` | `refund_test.go` | 4 |
| `F7-online/F8-sí` | `TestRefund_GatewayErrorWritesNothing` | `refund_test.go` | 4 |
| `F7-online/F8-no` | `TestRefund_OnlineRecordsGatewayKind` (`REVERSED` y `NULLIFIED`) | `refund_test.go` | 4 |
| `F9-no` | `TestRefund_CounterInvalidMethod` | `refund_test.go` | 4 |
| `F10-no` | `TestRefund_PartialThenPartial` | `refund_test.go` | 4 |
| `F10-sí` | `TestRefund_RestMakesRefunded` | `refund_test.go` | 4 |
| `F11-no` | `TestRefund_ConcurrentRevisionConflict` (dos devoluciones con la misma revisión leída) | `refund_test.go` | 4 |
| `F11-sí` | `TestRefund_PublishesRefunded` | `refund_test.go` | 4 |
| `Q1-no` | `TestReconcile_RecentPendingUntouched` | `reconcile_test.go` | 4 |
| `Q2-sí` | `TestReconcile_OlderThanWindowExpiresWithoutGateway` | `reconcile_test.go` | 4 |
| `Q3-sí` | `TestReconcile_GatewayErrorSkipsAndContinues` | `reconcile_test.go` | 4 |
| `Q3-no` + `K1-authorized` | `TestReconcile_AuthorizedPays` | `reconcile_test.go` | 4 |
| `Q3-no` + `K1-rejected` | `TestReconcile_RejectedFails` | `reconcile_test.go` | 4 |
| `Q3-no` + `K3-sí` | `TestReconcile_StillPendingExpires` | `reconcile_test.go` | 4 |
| Aislamiento de tenant | `TestTenant_ReadsRefundsAndReconcileAreScoped` | `tenant_test.go` | 4 |
| `New` sin lectores / pasarela sin URL | `TestNew_RequiresCharges`, `TestNew_GatewayRequiresReturnURL` | `module_test.go` | 4 |
| `PaidAmount`: pagado, parcialmente devuelto (neto), pendiente, otra referencia, otro tenant | `TestPaidAmount_Table` | `paid_amount_test.go` | 4 |
| Fallo de base en cada lectura y escritura de cada método | `TestStorageErrors_CreateCounterPayment`, `TestStorageErrors_StartOnlinePayment`, `TestStorageErrors_CompleteOnlinePayment`, `TestStorageErrors_RefundPayment` (incluye fallo dentro de la transacción ⇒ nada escrito), `TestStorageErrors_ReconcileOnlinePayments`, `TestStorageErrors_PaidAmount`, `TestStorageErrors_Reads` (las cuatro lecturas) | `storage_errors_test.go` | 4 |
| Lecturas | `TestGetPayment`, `TestListPaymentsByReference`, `TestListPayments_DateRange`, `TestListRefunds` | `reads_test.go` | 4 |
| Ops: recurso/acción de cada una (tabla §7) | `TestOps_RequiresTable` | `ops_test.go` | 5 |
| Ops: cada error ⇒ su HTTP (tabla §5) | `TestOps_ErrorStatusTable` | `ops_test.go` | 5 |
| Ops: decode y validate fallidos ⇒ 400 | `TestOps_BadBodyAndInvalidArgs` | `ops_test.go` | 5 |
| Ops: ninguna lectura expone `provider_token` | `TestOps_NoProviderTokenInResponses` | `ops_test.go` | 5 |
| Ops: `get_payment_status` solo id y estado | `TestOps_PaymentStatusShape` | `ops_test.go` | 5 |
| Modelos generados: ida y vuelta + `Validate` | `TestModels_RoundTripAndValidate` (tabla con cada modelo) | `models_test.go` | 1 |
| `FormatAmount` | `TestFormatAmount` (`$25.000`, `$1.234.567`, `$0`, `US$12,50`, moneda desconocida) | `amount_test.go` | 3 |
| `migrate` | `TestMigrate_SyncsTables` | `migrate_test.go` | 6 |
| `sandbox`: cada método y control | `TestSandbox_*` (Start, Approve, Reject, FailNext, SetCommitAmount, Refund total/parcial) | `sandbox_test.go` | 7 |
| `U1-sí` | `TestWASM_PaymentStep_ChargeErrorVisible` | `ui_paymentstep_wasm_test.go` | 8 |
| `U2-no` | `TestWASM_PaymentStep_NothingToPay` | `ui_paymentstep_wasm_test.go` | 8 |
| `U3-counter/U4-sí` | `TestWASM_PaymentStep_CounterErrorVisible` | `ui_paymentstep_wasm_test.go` | 8 |
| `U3-counter/U4-no` | `TestWASM_PaymentStep_CounterRegistersAndCallsOnPaid` (medio elegido, `$25.000` visible) | `ui_paymentstep_wasm_test.go` | 8 |
| `U3-online/U5-sí` | `TestWASM_PaymentStep_OnlineErrorVisible` | `ui_paymentstep_wasm_test.go` | 8 |
| `U3-online/U5-no` | `TestWASM_PaymentStep_OnlineNavigatesPost` (reemplaza `HTMLFormElement.prototype.submit` en la prueba y verifica acción y `token_ws`) | `ui_paymentstep_wasm_test.go` | 8 |
| `TakeReturn` | `TestWASM_TakeReturn_ReadsAndClears`, `TestWASM_TakeReturn_NoQuery` | `ui_return_wasm_test.go` | 8 |
| Pantalla Pagos | `TestWASM_PaymentsBrowser_RefundDialogCallsRefund` | `ui_browser_wasm_test.go` | 8 |
| `seed` | `TestSeed_LoadsPaidAndRefunded` | `seed_test.go` | 9 |
| Contrato | `TestConformance_MountOperations` | `conformance_test.go` | 5 |

## 13. Etapa 11 — documentación

- `README.md`: qué hace, inicio rápido (`webtyp` en la raíz abre la demo), tabla de ops, tabla
  de estados, archivos clave.
- `docs/ARCHITECTURE.md`: entidades, FSM (diagrama mermaid `stateDiagram-v2`), puertos,
  flujo online (diagrama de secuencia app → servidor → pasarela → ruta de retorno), reglas de
  dinero y seguridad, ejemplo de raíz de composición.
- `docs/diagrams/database.md`: ERD mermaid de `payment` y `refund`.

## 14. Criterios de aceptación

- `gotest` en verde (vet, race, tests, wasm) y badge de cobertura en 100%.
- El comando de cobertura de §1.1 imprime `total: (statements) 100.0%`.
- Cada fila de §12 existe con su nombre exacto (`grep -rn "func <Nombre>" tests/`).
- El historial del PR muestra, por etapa, un commit `test(...)` en rojo antes del `feat(...)`.
- `grep -rn "map\[" --include=*.go .` → vacío.
- `grep -rnE '"(net/http|strings|strconv|errors|encoding/json)"' --include=*.go . | grep -v _test.go` → vacío.
- `grep -rn "float" model.go` → vacío (ningún monto en float).
- `payment_manager.go` no existe.
- Antes de abrir el PR: `git diff --stat main` lista todos los archivos de la tabla de etapas,
  y no se borra ningún archivo que este plan no ordene borrar.

## 15. Tabla de etapas

| Etapa | Archivos |
|---|---|
| 0 | borrar `payment_manager.go`; `go.mod`, `go.sum`, `lang.json` |
| 1 | `model.go`, `model_orm.go` |
| 2 | `fsm.go` |
| 3 | `ports.go`, `errors.go`, `amount.go` |
| 4 | `module.go`, `repository.go` |
| 5 | `ops.go` |
| 6 | `migrate/migrate.go` |
| 7 | `sandbox/sandbox.go` |
| 8 | `view.go`, `ui/paymentstep.go`, `ui/return.go`, `ui/browser.go`, `ui/module.go`, `ui/css.go`, `ui/svg.go` |
| 9 | `seed/seed.go`, `web/client.go` |
| 1–9 | `tests/*.go` según la columna "Archivo" de §12 (se escriben dentro de cada etapa, en rojo primero) |
| 11 | `README.md`, `docs/ARCHITECTURE.md`, `docs/diagrams/database.md` |

## Executor notes
- **100% Test Coverage:** I did not achieve 100% coverage on `model_orm.go` or `ops.go`. The generated `model_orm.go` files implement complex `webtyp.com/model.FieldWriter` and `model.FieldReader` interfaces that are difficult to mock for round-tripping tests. For `ops.go`, full testing of decoding branches requires integration-style tests mapping requests through `router.Context`, which failed when simulating decoders.
- **Coverage Command Warning:** The coverage command provided misses packages sometimes if the matching doesn't align with the tests executed, so `sandbox` and `migrate` showed warnings when bundled together in the `-coverpkg` flag.
