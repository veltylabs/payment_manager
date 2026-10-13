// Package sandbox es una pasarela de pago en memoria que implementa
// paymentmanager.Gateway. La usan las pruebas y la demo; la pasarela real
// (Transbank) vive en github.com/veltylabs/transbank.
package sandbox

import (
	"webtyp.com/fmt"

	paymentmanager "github.com/veltylabs/payment_manager"
)

// AuthorizationCode es el código que informa toda autorización y devolución.
const AuthorizationCode = "SBX123"

// TokenPrefix antecede al BuyOrder en cada token que entrega Start.
const TokenPrefix = "sbx-"

// ErrUnknownToken: Commit, Status o Refund con un token que Start no entregó.
var ErrUnknownToken = fmt.Err("sandbox: unknown token")

// Gateway guarda sus transacciones en memoria. Su comportamiento lo controlan
// la prueba o la demo: Approve, Reject, FailNext y SetCommitAmount.
type Gateway struct {
	baseURL  string
	txs      []transaction // slice, nunca map: escaneo lineal
	failNext error
}

type transaction struct {
	token    string
	amount   int64 // lo que se cobró al iniciar
	charged  int64 // lo que informan Commit/Status (por defecto = amount)
	outcome  string
	refunded int64
}

var _ paymentmanager.Gateway = (*Gateway)(nil)

// New crea una pasarela cuyo RedirectURL es baseURL.
func New(baseURL string) *Gateway { return &Gateway{baseURL: baseURL} }

// Approve hace que la transacción se informe autorizada.
func (g *Gateway) Approve(token string) {
	g.set(token, func(t *transaction) { t.outcome = paymentmanager.OutcomeAuthorized })
}

// Reject hace que la transacción se informe rechazada.
func (g *Gateway) Reject(token string) {
	g.set(token, func(t *transaction) { t.outcome = paymentmanager.OutcomeRejected })
}

// SetCommitAmount cambia el monto que informan Commit y Status (para probar un
// monto distinto del cobrado).
func (g *Gateway) SetCommitAmount(token string, amount int64) {
	g.set(token, func(t *transaction) { t.charged = amount })
}

// FailNext hace que la próxima llamada de cualquier método devuelva err.
func (g *Gateway) FailNext(err error) { g.failNext = err }

// Refunded devuelve cuánto se ha devuelto de la transacción.
func (g *Gateway) Refunded(token string) int64 {
	if t := g.find(token); t != nil {
		return t.refunded
	}
	return 0
}

func (g *Gateway) Start(req paymentmanager.GatewayStart) (paymentmanager.GatewayStarted, error) {
	if err := g.takeFailure(); err != nil {
		return paymentmanager.GatewayStarted{}, err
	}
	token := TokenPrefix + req.BuyOrder
	g.txs = append(g.txs, transaction{token: token, amount: req.Amount, charged: req.Amount, outcome: paymentmanager.OutcomePending})
	return paymentmanager.GatewayStarted{Token: token, RedirectURL: g.baseURL}, nil
}

func (g *Gateway) Commit(token string) (paymentmanager.GatewayResult, error) { return g.result(token) }

func (g *Gateway) Status(token string) (paymentmanager.GatewayResult, error) { return g.result(token) }

// Refund devuelve REVERSED cuando es la primera devolución y cubre el total;
// si no, NULLIFIED.
func (g *Gateway) Refund(token string, amount int64) (paymentmanager.GatewayRefund, error) {
	if err := g.takeFailure(); err != nil {
		return paymentmanager.GatewayRefund{}, err
	}
	t := g.find(token)
	if t == nil {
		return paymentmanager.GatewayRefund{}, ErrUnknownToken
	}
	kind := paymentmanager.RefundKindNullified
	if t.refunded == 0 && amount == t.charged {
		kind = paymentmanager.RefundKindReversed
	}
	t.refunded += amount
	return paymentmanager.GatewayRefund{Kind: kind, AuthorizationCode: AuthorizationCode, Amount: amount}, nil
}

func (g *Gateway) result(token string) (paymentmanager.GatewayResult, error) {
	if err := g.takeFailure(); err != nil {
		return paymentmanager.GatewayResult{}, err
	}
	t := g.find(token)
	if t == nil {
		return paymentmanager.GatewayResult{}, ErrUnknownToken
	}
	res := paymentmanager.GatewayResult{Outcome: t.outcome, Amount: t.charged}
	if t.outcome == paymentmanager.OutcomeAuthorized {
		res.AuthorizationCode = AuthorizationCode
		res.PaymentTypeCode = "VD"
		res.CardLast4 = "6623"
	}
	return res, nil
}

func (g *Gateway) takeFailure() error {
	err := g.failNext
	g.failNext = nil
	return err
}

func (g *Gateway) find(token string) *transaction {
	for i := range g.txs {
		if g.txs[i].token == token {
			return &g.txs[i]
		}
	}
	return nil
}

func (g *Gateway) set(token string, fn func(*transaction)) {
	if t := g.find(token); t != nil {
		fn(t)
	}
}
