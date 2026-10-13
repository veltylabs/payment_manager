# Architecture - Payment Manager

Este módulo gestiona la persistencia, estados y conciliación de pagos.
Está diseñado para ser la fuente de la verdad financiera de pagos (mesón/online).

## Entidades Principales

- `Payment`: Registro de pago, con un id único, apuntando a una referencia (`ReferenceModule`/`ReferenceId`).
- `Refund`: Una devolución (reversa o anulación) efectuada sobre un `Payment`.

## Máquina de Estados (FSM)

```mermaid
stateDiagram-v2
    [*] --> PENDING: StartOnlinePayment
    [*] --> PAID: CreateCounterPayment
    PENDING --> PAID: PAY
    PENDING --> FAILED: FAIL
    PENDING --> CANCELLED: CANCEL
    PENDING --> EXPIRED: EXPIRE
    PAID --> PARTIALLY_REFUNDED: REFUND_PARTIAL
    PAID --> REFUNDED: REFUND_TOTAL
    PARTIALLY_REFUNDED --> PARTIALLY_REFUNDED: REFUND_PARTIAL
    PARTIALLY_REFUNDED --> REFUNDED: REFUND_TOTAL
```

## Flujo Online (Webpay)

1. **Cliente / App** llama a `start_online_payment`.
2. Servidor pide `Gateway.Start` (por ejemplo Transbank). Devuelve token y URL.
3. El frontend envía POST form `token_ws` a la URL (navegador abandona la App).
4. El proveedor redirige al retorno de Velty Labs (`CompleteOnlinePayment`).
5. Servidor verifica el estado con `Gateway.Commit(token)`.
6. Si hay discrepancia de montos, se emite un `Refund` inmediato hacia la pasarela, evitando cobros indebidos.

## Reglas Strictas de Dinero

- **Nunca de entrada de cliente**: El valor se consulta siempre al módulo dueño (e.g. `appointment_booking` vía `ChargeReader`).
- **Enteros y Monedas**: Siempre se almacena el monto en las unidades mínimas indivisibles (`int64`). CLP sin decimales, USD con 2.
- **Concurrencia optimista**: cada cambio de un pago (cierre, devolución) relee el pago dentro de la transacción y compara `revision`; si cambió, responde `ErrConflict` y no escribe nada. La devolución crea el `Refund` y actualiza el pago en la misma transacción.
- **Ids de quien llama**: pagos y devoluciones traen su id; repetir la misma operación devuelve lo guardado y reusar un id para otra cosa (aun en otro tenant) responde `ErrIdTaken`.
- **Lo que devuelve la pasarela se valida** antes de guardarse (código de autorización, últimos 4 dígitos).

## Puertos

- `Gateway` (`Start`, `Commit`, `Status`, `Refund`): la pasarela concreta (`github.com/veltylabs/transbank`) la inyecta la app; `sandbox/` es la versión en memoria.
- `ChargeReader`: el módulo dueño de la referencia responde cuánto se cobra (`Deps.Charges`).
- `PaidAmount(tenant, paymentId, module, reference)`: lo usa el módulo dueño para verificar un pago antes de confirmar (lo satisface `*Module` estructuralmente).

## Eventos

`payment.paid` y `payment.failed` (payload `*Payment`), `payment.refunded` (payload `*Refund`).

Todos los flujos, con un id por rama y la prueba que cubre cada una: [diagrams/flows.md](diagrams/flows.md).
