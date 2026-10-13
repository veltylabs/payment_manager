package paymentmanager

// Eventos de la máquina de estados del pago.
const (
	EventPay           = "PAY"
	EventFail          = "FAIL"
	EventCancel        = "CANCEL"
	EventExpire        = "EXPIRE"
	EventRefundPartial = "REFUND_PARTIAL"
	EventRefundTotal   = "REFUND_TOTAL"
)

// transition es una fila de la tabla: (estado, evento) -> estado siguiente.
// Slice, nunca map (AGENTS.md); ocho filas, escaneo lineal.
type transition struct {
	from, event, to string
}

var transitions = []transition{
	{StatusPending, EventPay, StatusPaid},
	{StatusPending, EventFail, StatusFailed},
	{StatusPending, EventCancel, StatusCancelled},
	{StatusPending, EventExpire, StatusExpired},
	{StatusPaid, EventRefundPartial, StatusPartiallyRefunded},
	{StatusPaid, EventRefundTotal, StatusRefunded},
	{StatusPartiallyRefunded, EventRefundPartial, StatusPartiallyRefunded},
	{StatusPartiallyRefunded, EventRefundTotal, StatusRefunded},
}

// Transition devuelve el estado al que lleva event desde current, o
// ErrInvalidTransition.
func Transition(current, event string) (string, error) {
	for _, t := range transitions {
		if t.from == current && t.event == event {
			return t.to, nil
		}
	}
	return "", ErrInvalidTransition
}
