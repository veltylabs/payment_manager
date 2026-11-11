package paymentmanager

const (
	DefaultTerminalPushSeconds      = 5  // the machine must accept the charge or the cashier is told «no connection»
	DefaultTerminalPayWindowSeconds = 30 // how long the customer has to pass the card before the payment is offered for manual confirmation
	DefaultOnlineStartSeconds       = 10 // the gateway must answer with the payment link
	TerminalPushCapSeconds          = 10 // Definition caps: Timeouts above them are refused by New
	OnlineStartCapSeconds           = 15
)

// Timeouts are the waits of the module. A zero field takes its default. Example: Deps{Timeouts: Timeouts{TerminalPayWindowSeconds: 60}}
type Timeouts struct {
	TerminalPushSeconds      int64
	TerminalPayWindowSeconds int64
	OnlineStartSeconds       int64
}

func (t Timeouts) withDefaults() (Timeouts, error) {
	if t.TerminalPushSeconds == 0 { t.TerminalPushSeconds = DefaultTerminalPushSeconds }
	if t.TerminalPayWindowSeconds == 0 { t.TerminalPayWindowSeconds = DefaultTerminalPayWindowSeconds }
	if t.OnlineStartSeconds == 0 { t.OnlineStartSeconds = DefaultOnlineStartSeconds }
	if t.TerminalPushSeconds < 0 || t.TerminalPayWindowSeconds < 0 || t.OnlineStartSeconds < 0 ||
		t.TerminalPushSeconds > TerminalPushCapSeconds || t.OnlineStartSeconds > OnlineStartCapSeconds {
		return Timeouts{}, ErrTimeoutOutOfRange
	}
	return t, nil
}
