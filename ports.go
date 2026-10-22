package paymentmanager
import "webtyp.com/time"

// Terminal es la máquina de cobro con tarjeta de una caja. La implementación concreta (por ejemplo
// github.com/veltylabs/tuu) la inyecta la app; sus llamadas son síncronas y solo corren en el servidor.
type Terminal interface {
	Provider() string                               // nombre del procesador; se guarda en cada pago ("tuu")
	Push(req TerminalPush) (TerminalPushed, error)  // envía el monto a la máquina; idempotente por RequestId
	Status(token string) (TerminalResult, error)    // estado del cobro en la máquina
}

type TerminalPush struct {
	RequestId   string // = Payment.Id; la máquina lo usa como llave de idempotencia
	Device      string // id de la máquina en el procesador (TUU: número de serie)
	Amount      int64
	Currency    string
	Method      string // MethodDebit | MethodCredit
	Description string // texto corto que muestra la máquina; puede ir vacío
}

type TerminalPushed struct {
	Token string // id del cobro en el procesador; se guarda en Payment.ProviderToken
}

type TerminalResult struct {
	Outcome           string // OutcomePending | OutcomeAuthorized | OutcomeRejected | OutcomeCancelled
	Amount            int64
	AuthorizationCode string
	CardLast4         string
}

// Gateway es la pasarela de pago en línea. La implementación (github.com/veltylabs/transbank,
// github.com/veltylabs/tuu) la inyecta la app; verifica sola lo que le devuelve su procesador y llama a
// CompleteOnlinePayment con el resultado ya verificado. Sus llamadas son síncronas y solo corren en el servidor.
type Gateway interface {
	Provider() string
	Start(req GatewayStart) (GatewayStarted, error)
	Status(token string) (GatewayResult, error)
	Refund(token string, amount int64) (GatewayRefund, error)
}

type GatewayStart struct {
	PaymentId   string // nuestro id (Webpay: buy_order y session_id; TUU: x_reference)
	Amount      int64
	Currency    string
	Description string // texto corto para el pagador; puede ir vacío
	Payer       Payer  // lo exige TUU; Webpay lo ignora
}

// Payer es quien paga en línea. Los campos pueden ir vacíos; un adaptador que los exige responde error.
type Payer struct {
	Name  string
	Email string
	Phone string
}

type GatewayStarted struct {
	Token       string // id del cobro en el procesador; se guarda en Payment.ProviderToken
	RedirectURL string
	TokenField  string // no vacío: el navegador envía Token por POST en este campo (Webpay: "token_ws"); vacío: GET a RedirectURL
}

const (
	OutcomePending    = "pending"
	OutcomeAuthorized = "authorized"
	OutcomeRejected   = "rejected"
	OutcomeCancelled  = "cancelled"
	OutcomeExpired    = "expired"
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
	Kind              string
	AuthorizationCode string
	Amount            int64
}

type ChargeReader interface {
	Charge(tenantId, referenceId string) (amount int64, currency string, payable bool, err error)
}

type ChargeSource struct {
	Module string
	Reader ChargeReader
	Device string // id de la máquina de la caja que cobra las referencias de este módulo; "" = sin máquina
}

// OnlineResult es lo que el adaptador ya verificó (firma del aviso o confirmación con el procesador).
type OnlineResult struct {
	PaymentId string
	Result    GatewayResult
}

// PayoutTerms es lo que un adaptador (Terminal o Gateway) sabe de su plan contratado: cuánto descuenta el
// procesador y en cuántos días hábiles bancarios deposita. Es opcional: el módulo lo detecta con una aserción
// de tipo; sin él, un cobro con tarjeta se registra sin fecha de depósito ni comisión.
// Ejemplo: terms, ok := gateway.(paymentmanager.PayoutTerms)
type PayoutTerms interface {
	Payout(c PayoutCharge) (Payout, error)
}

// PayoutCharge es el cobro del que se pregunta el abono.
type PayoutCharge struct {
	Amount   int64
	Currency string
	Method   string // MethodDebit, MethodCredit o MethodCard
	Channel  string // ChannelTerminal o ChannelOnline
}

// Payout es lo que responde el adaptador: comisión, IVA de la comisión y días hábiles hasta el depósito.
// El neto depositado es Amount − Fee − FeeTax.
type Payout struct {
	Fee          int64
	FeeTax       int64
	BusinessDays int
}

// BankDays convierte días hábiles bancarios en fecha. Lo cumple business_calendar.(*Module).BankDayAfter.
// Ejemplo: deps.BankDays = calendario
type BankDays interface {
	BankDayAfter(from time.Unix, days int) (time.Unix, error)
}
