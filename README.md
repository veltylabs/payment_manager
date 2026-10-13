# Payment Manager

Módulo de dominio para registrar pagos en mesón (efectivo, tarjeta) y pagos en línea (Webpay Plus). Maneja también devoluciones y conciliación con la pasarela.

## Inicio rápido

Ejecuta el siguiente comando para abrir la aplicación de demostración en tu navegador:

```bash
webtyp
```

## Operaciones

| Op | Descripción | Require |
|---|---|---|
| `get_charge` | Consulta el valor a pagar y su moneda | `payment_online`, `Read` |
| `create_counter_payment` | Registra un pago de mesón | `payment`, `Create` |
| `start_online_payment` | Inicia flujo webpay | `payment_online`, `Create` |
| `get_payment_status` | Estado actual de un pago | `payment_online`, `Read` |
| `get_payment` | Obtiene un pago (limpio de token) | `payment`, `Read` |
| `list_payments_by_reference` | Pagos de una reserva/referencia | `payment`, `Read` |
| `list_payments` | Lista pagos | `payment`, `Read` |
| `refund_payment` | Realiza una devolución parcial o total | `refund`, `Create` |
| `list_refunds` | Devoluciones de un pago | `refund`, `Read` |
| `reconcile_online_payments` | Cierra pagos online perdidos | `payment`, `Update` |

## Estados

| Estado | Descripción |
|---|---|
| `PENDING` | Pago online iniciado, esperando respuesta de pasarela |
| `PAID` | Pago completado y acreditado |
| `FAILED` | Pago rechazado por la pasarela |
| `CANCELLED` | El cliente abortó o canceló el flujo webpay |
| `EXPIRED` | El pago webpay no fue completado a tiempo |
| `PARTIALLY_REFUNDED` | Se devolvió parte del monto, pero queda saldo |
| `REFUNDED` | Devolución total del monto |
