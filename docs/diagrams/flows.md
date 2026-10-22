# Flujos de `payment_manager`

Cada rombo lleva un id (`C1`, `S2`, …). Cada rama tiene una prueba en `tests/` con su id en el nombre o en un comentario.

## 1. Estados del pago (FSM)

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
    FAILED --> [*]
    CANCELLED --> [*]
    EXPIRED --> [*]
    REFUNDED --> [*]
```

## 2. Resolver el monto (`Charge`)

```mermaid
flowchart TD
    A[Charge tenant, módulo, referencia] --> H1{H1: ¿hay ChargeSource<br>para el módulo?}
    H1 -- no --> E1[ErrUnknownReference]
    H1 -- sí --> L[Reader.Charge]
    L --> H2{H2: ¿error del lector?}
    H2 -- sí --> E2[devolver el error]
    H2 -- no --> H3{H3: ¿moneda CLP o USD?}
    H3 -- no --> E3[ErrUnsupportedCurrency]
    H3 -- sí --> OK[monto, moneda, payable]
```

## 3. Pago de mesón (`CreateCounterPayment`)

```mermaid
flowchart TD
    A[CreateCounterPayment] --> C1{C1: ¿faltan campos?}
    C1 -- sí --> E0[ErrMissingArgs]
    C1 -- no --> C2{C2: ¿existe un pago con ese id?}
    C2 -- sí --> C3{C3: ¿mismo tenant y<br>misma referencia?}
    C3 -- sí --> R0[devolver el guardado<br>sin evento]
    C3 -- no --> E1[ErrIdTaken]
    C2 -- no --> C4{C4: ¿medio cash/debit/<br>credit/transfer?}
    C4 -- no --> E2[ErrInvalidMethod]
    C4 -- sí --> C5{C5: ¿Due > 0?}
    C5 -- sí --> E3[ErrAlreadyPaid]
    C5 -- no --> CH[Charge §2]
    CH --> C6{C6: ¿payable?}
    C6 -- no --> E4[ErrNotPayable]
    C6 -- sí --> W[crear PAID, revision 1]
    W --> EV[publicar payment.paid]
```

## 4. Iniciar pago online (`StartOnlinePayment`)

```mermaid
flowchart TD
    A[StartOnlinePayment] --> S1{S1: ¿Gateway configurado?}
    S1 -- no --> E0[ErrGatewayUnavailable]
    S1 -- sí --> S2{S2: ¿faltan campos?}
    S2 -- sí --> E1[ErrMissingArgs]
    S2 -- no --> S3{S3: ¿id ya usado?}
    S3 -- sí --> E2[ErrIdTaken]
    S3 -- no --> S4{S4: ¿referencia ya pagada?}
    S4 -- sí --> E3[ErrAlreadyPaid]
    S4 -- no --> CH[Charge §2]
    CH --> S5{S5: ¿payable?}
    S5 -- no --> E4[ErrNotPayable]
    S5 -- sí --> P[crear PENDING online/webpay]
    P --> G[Gateway.Start]
    G --> S6{S6: ¿error de la pasarela?}
    S6 -- sí --> F[pasar a FAILED] --> E5[ErrGatewayFailed]
    S6 -- no --> T[guardar token] --> OK[payment_id, redirect_url, token]
```

## 5. Volver de la pasarela (`CompleteOnlinePayment`)

```mermaid
flowchart TD
    A[CompleteOnlinePayment ret] --> R1{R1: ¿qué trajo?}
    R1 -- AbortToken --> R2{R2: ¿pago con ese token?}
    R2 -- no --> NF1[ErrNotFound]
    R2 -- sí --> R3{R3: ¿PENDING?}
    R3 -- no --> SAME1[devolver tal cual]
    R3 -- sí --> CAN[CANCEL]
    R1 -- solo BuyOrder --> R4{R4: ¿pago con ese id?}
    R4 -- no --> NF2[ErrNotFound]
    R4 -- sí --> R5{R5: ¿PENDING?}
    R5 -- no --> SAME2[devolver tal cual]
    R5 -- sí --> EXP[EXPIRE]
    R1 -- Token --> R6{R6: ¿pago con ese token?}
    R6 -- no --> NF3[ErrNotFound]
    R6 -- sí --> R7{R7: ¿PENDING?}
    R7 -- no --> SAME3[devolver tal cual<br>retorno repetido]
    R7 -- sí --> CM[Gateway.Commit]
    CM --> R8{R8: ¿error de la pasarela?}
    R8 -- sí --> KEEP[queda PENDING<br>ErrGatewayFailed]
    R8 -- no --> SETTLE[Asentar §6]
    R1 -- nada --> E0[ErrMissingArgs]
```

## 6. Asentar un resultado de la pasarela (lo usan §5 y §8)

```mermaid
flowchart TD
    A[resultado] --> K1{K1: ¿outcome?}
    K1 -- authorized --> K2{K2: ¿monto igual<br>al del pago?}
    K2 -- sí --> PAY[PAY + datos de tarjeta<br>publicar payment.paid]
    K2 -- no --> RF[Gateway.Refund del monto cobrado]
    RF --> FAIL1[FAIL<br>ErrAmountMismatch]
    K1 -- rejected --> FAIL2[FAIL<br>publicar payment.failed]
    K1 -- pending --> K3{K3: ¿lo llama la conciliación?}
    K3 -- sí --> EXP[EXPIRE]
    K3 -- no --> KEEP[queda PENDING]
```

## 7. Devolver (`RefundPayment`)

```mermaid
flowchart TD
    A[RefundPayment] --> F1{F1: ¿faltan campos?}
    F1 -- sí --> E0[ErrMissingArgs]
    F1 -- no --> F2{F2: ¿existe devolución con ese id?}
    F2 -- sí --> F3{F3: ¿mismo tenant y pago?}
    F3 -- sí --> R0[devolver la guardada]
    F3 -- no --> E1[ErrIdTaken]
    F2 -- no --> F4{F4: ¿pago del tenant?}
    F4 -- no --> E2[ErrNotFound]
    F4 -- sí --> F5{F5: ¿PAID o<br>PARTIALLY_REFUNDED?}
    F5 -- no --> E3[ErrInvalidTransition]
    F5 -- sí --> F6{F6: ¿0 < monto ≤ saldo?}
    F6 -- no --> E4[ErrRefundExceeds]
    F6 -- sí --> F7{F7: ¿canal?}
    F7 -- online --> G[Gateway.Refund]
    G --> F8{F8: ¿error?}
    F8 -- sí --> E5[ErrGatewayFailed<br>nada escrito]
    F8 -- no --> W
    F7 -- counter --> F9{F9: ¿medio válido?}
    F9 -- no --> E6[ErrInvalidMethod]
    F9 -- sí --> W[Tx: crear Refund,<br>sumar RefundedAmount]
    W --> F10{F10: ¿saldo queda en 0?}
    F10 -- sí --> TOT[REFUND_TOTAL]
    F10 -- no --> PAR[REFUND_PARTIAL]
    TOT --> F11{F11: ¿revision coincide?}
    PAR --> F11
    F11 -- no --> E7[ErrConflict<br>rollback]
    F11 -- sí --> EV[publicar payment.refunded]
```

## 8. Conciliar (`ReconcilePayments`)

```mermaid
flowchart TD
    A[para cada pago online/máquina PENDING del tenant] --> Q1{Q1: ¿creado hace más de<br>PendingTimeout?}
    Q1 -- no --> SKIP[no tocar]
    Q1 -- sí --> Q2{Q2: ¿más viejo que<br>GatewayStatusWindow?}
    Q2 -- sí --> EXP[EXPIRE sin llamar a la pasarela o máquina]
    Q2 -- no --> ST[Gateway.Status o Terminal.Status]
    ST --> Q3{Q3: ¿error?}
    Q3 -- sí --> NEXT[saltarlo y seguir]
    Q3 -- no --> SETTLE[Asentar §6 con K3 = sí]
```

## 9. Flujo online completo (secuencia)

```mermaid
sequenceDiagram
    participant N as Navegador (ui.PaymentStep)
    participant S as Servidor (payment_manager)
    participant T as transbank (ruta de retorno)
    participant W as Webpay
    N->>S: start_online_payment(id, referencia, return_to)
    S->>S: Charge (monto del módulo dueño)
    S->>W: Gateway.Start
    W-->>S: token, url
    S-->>N: redirect_url, token
    N->>W: dom.NavigatePost(url, token_ws)
    W->>T: GET/POST return_url (token_ws | TBK_TOKEN | TBK_ORDEN_COMPRA)
    T->>S: CompleteOnlinePayment(OnlineReturn)
    S->>W: Gateway.Commit(token)
    W-->>S: AUTHORIZED / FAILED
    S-->>T: Payment
    T-->>N: 303 /?payment=id&status=PAID#return_to
    N->>N: ui.TakeReturn()
```

## 11. Cobro con máquina (StartTerminalPayment / RefreshTerminalPayment)

```mermaid
flowchart TD
    A[Push] --> T1{Push OK, Status authorized} --> P1[PAID, EventPay]
    A --> T2{Status rejected} --> P2[FAILED, EventFail]
    A --> T3{Status cancelled} --> P3[CANCELLED, EventCancel]
    A --> T4{Status pending} --> P4[PENDING]
    A --> T5{Authorized monto menor} --> P5[FAILED, ErrAmountMismatch]
    A --> T6{Push error} --> P6[FAILED, ErrTerminalFailed]
    A --> T7{Status error} --> P7[PENDING, ErrTerminalFailed]
    B[CreateCounterPayment] --> T8{Module with device} --> P8[ErrTerminalRequired]
    A --> T9{Same ID twice} --> P9[Returns first, one Push]
    C[RefreshTerminalPayment] --> T10{Counter payment} --> P10[ErrNotTerminal]
```

## 10. Paso de pago en pantalla (`ui.PaymentStep`)

```mermaid
flowchart TD
    A[Init] --> U1{U1: ¿get_charge falló?}
    U1 -- sí --> ERR0[mensaje de error visible]
    U1 -- no --> U2{U2: ¿payable?}
    U2 -- no --> NP[Nada que pagar]
    U2 -- sí --> U3{U3: ¿canal?}
    U3 -- counter --> CNT[medio + comprobante + Registrar pago]
    CNT --> U4{U4: ¿create_counter_payment falló?}
    U4 -- sí --> ERR1[mensaje visible, botón habilitado]
    U4 -- no --> PAID[OnPaid id]
    U3 -- online --> ONL[Pagar con Webpay]
    ONL --> U5{U5: ¿start_online_payment falló?}
    U5 -- sí --> ERR2[mensaje visible, botón habilitado]
    U5 -- no --> NAV[NavigatePost url, token_ws]
```
