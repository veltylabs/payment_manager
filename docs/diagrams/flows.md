# Flujos de pago

## Generación de hechos contables (`paymentFacts`)

```mermaid
flowchart TD
    A[Inicio: pago pagado] --> B{¿Es efectivo o transferencia?}
    B -- Sí --> C[Emitir Charged Entrada]
    B -- No (Tarjeta) --> D[Armar Accrued Entrada Base]
    D --> E{¿El adaptador implementa PayoutTerms?}
    E -- No --> F[Emitir Accrued Entrada (DueAt = 0)]
    E -- Sí --> G[Calcular Payout y DueAt con BankDays]
    G --> H[Fijar DueAt en Accrued Entrada Base]
    H --> I{¿Comisión > 0?}
    I -- No --> J[Emitir Accrued Entrada]
    I -- Sí --> K[Emitir Accrued Entrada + Accrued Salida (payment_fee)]
```
