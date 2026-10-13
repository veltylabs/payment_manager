package paymentmanager

const (
	EventPay           = "PAY"
	EventFail          = "FAIL"
	EventCancel        = "CANCEL"
	EventExpire        = "EXPIRE"
	EventRefundPartial = "REFUND_PARTIAL"
	EventRefundTotal   = "REFUND_TOTAL"
)

func Transition(current, event string) (string, error) {
	type transition struct {
		From  string
		Event string
	}
	transitions := map[transition]string{
		{StatusPending, EventPay}:                 StatusPaid,
		{StatusPending, EventFail}:                StatusFailed,
		{StatusPending, EventCancel}:              StatusCancelled,
		{StatusPending, EventExpire}:              StatusExpired,
		{StatusPaid, EventRefundPartial}:          StatusPartiallyRefunded,
		{StatusPaid, EventRefundTotal}:            StatusRefunded,
		{StatusPartiallyRefunded, EventRefundPartial}: StatusPartiallyRefunded,
		{StatusPartiallyRefunded, EventRefundTotal}:   StatusRefunded,
	}

	next, ok := transitions[transition{From: current, Event: event}]
	if !ok {
		return "", ErrInvalidTransition
	}
	return next, nil
}
