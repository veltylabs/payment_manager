package paymentmanager

import "webtyp.com/model"

type Gateway interface {
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

func (s *GatewayStarted) DecodeFields(r model.FieldReader) {}
func (s *GatewayStarted) IsNil() bool { return false }
func (s *GatewayStarted) EncodeFields(w model.FieldWriter) {}
func (s *GatewayStarted) Validate(action byte) error { return nil }
func (s *GatewayStarted) ModelName() string { return "GatewayStarted" }
func (s *GatewayStarted) Schema() []model.Field { return nil }
func (s *GatewayStarted) Pointers() []string { return nil }

const (
	OutcomePending    = "pending"
	OutcomeAuthorized = "authorized"
	OutcomeRejected   = "rejected"
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
}

type OnlineReturn struct {
	Token      string
	AbortToken string
	BuyOrder   string
}
