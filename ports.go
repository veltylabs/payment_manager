package paymentmanager

// Gateway es la pasarela de pago online. La implementación concreta
// (github.com/veltylabs/transbank) la inyecta la app; sus llamadas son
// síncronas y solo corren en el servidor.
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

type Gateway interface {
	Provider() string
	Start(req GatewayStart) (GatewayStarted, error)
	Commit(token string) (GatewayResult, error)
	Status(token string) (GatewayResult, error)
	Refund(token string, amount int64) (GatewayRefund, error)
}

type GatewayStart struct {
	BuyOrder  string
	SessionId string
	Amount    int64
	Currency  string
	ReturnURL string
}

type GatewayStarted struct {
	Token       string
	RedirectURL string
}

const (
	OutcomePending    = "pending"
	OutcomeAuthorized = "authorized"
	OutcomeRejected   = "rejected"
	OutcomeCancelled  = "cancelled"
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

type OnlineReturn struct {
	Token      string
	AbortToken string
	BuyOrder   string
}
