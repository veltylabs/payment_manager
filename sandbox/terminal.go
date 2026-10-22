package sandbox

import (
	paymentmanager "github.com/veltylabs/payment_manager"
)

// Terminal es una máquina de tarjeta en memoria que implementa paymentmanager.Terminal.
type Terminal struct {
	txs      []terminalTx
	failNext error
}

type terminalTx struct {
	token    string
	amount   int64
	charged  int64
	outcome  string
}

const TerminalProvider = "sandbox"
const TerminalTokenPrefix = "sbt-"

var _ paymentmanager.Terminal = (*Terminal)(nil)

func NewTerminal() *Terminal {
	return &Terminal{}
}

func (t *Terminal) Provider() string {
	return TerminalProvider
}

func (t *Terminal) Push(req paymentmanager.TerminalPush) (paymentmanager.TerminalPushed, error) {
	if err := t.takeFailure(); err != nil {
		return paymentmanager.TerminalPushed{}, err
	}
	token := TerminalTokenPrefix + req.RequestId
	existing := t.find(token)
	if existing == nil {
		t.txs = append(t.txs, terminalTx{token: token, amount: req.Amount, charged: req.Amount, outcome: paymentmanager.OutcomePending})
	}
	return paymentmanager.TerminalPushed{Token: token}, nil
}

func (t *Terminal) Status(token string) (paymentmanager.TerminalResult, error) {
	if err := t.takeFailure(); err != nil {
		return paymentmanager.TerminalResult{}, err
	}
	tx := t.find(token)
	if tx == nil {
		return paymentmanager.TerminalResult{}, ErrUnknownToken
	}
	res := paymentmanager.TerminalResult{Outcome: tx.outcome, Amount: tx.charged}
	if tx.outcome == paymentmanager.OutcomeAuthorized {
		res.AuthorizationCode = AuthorizationCode
		res.CardLast4 = "4242"
	}
	return res, nil
}

func (t *Terminal) Approve(token string) {
	t.set(token, func(tx *terminalTx) { tx.outcome = paymentmanager.OutcomeAuthorized })
}

func (t *Terminal) Reject(token string) {
	t.set(token, func(tx *terminalTx) { tx.outcome = paymentmanager.OutcomeRejected })
}

func (t *Terminal) Cancel(token string) {
	t.set(token, func(tx *terminalTx) { tx.outcome = paymentmanager.OutcomeCancelled })
}

func (t *Terminal) SetAmount(token string, amount int64) {
	t.set(token, func(tx *terminalTx) { tx.charged = amount })
}

func (t *Terminal) FailNext(err error) {
	t.failNext = err
}

func (t *Terminal) Pushes() int {
	return len(t.txs)
}

func (t *Terminal) takeFailure() error {
	err := t.failNext
	t.failNext = nil
	return err
}

func (t *Terminal) find(token string) *terminalTx {
	for i := range t.txs {
		if t.txs[i].token == token {
			return &t.txs[i]
		}
	}
	return nil
}

func (t *Terminal) set(token string, fn func(*terminalTx)) {
	if tx := t.find(token); tx != nil {
		fn(tx)
	}
}
