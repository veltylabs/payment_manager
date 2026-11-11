package tests

import (
	"testing"

	paymentmanager "github.com/veltylabs/payment_manager"
	"github.com/veltylabs/payment_manager/sandbox"
	"webtyp.com/crud"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/router"
	"webtyp.com/router/mock"
	"webtyp.com/storage/mem"
	"webtyp.com/unixid"
)

const refModule = "booking"

// fakeCharge is the ChargeReader of the module that owns the reference: there is always 25.000 to pay.
type fakeCharge struct{}

func (fakeCharge) Charge(tenantId, referenceId string) (int64, string, bool, error) {
	return 25000, "CLP", true, nil
}

// newModule builds the module over an in-memory database; term may be nil (no card machine).
func newModule(t *testing.T, term paymentmanager.Terminal, timeouts paymentmanager.Timeouts) (*paymentmanager.Module, error) {
	t.Helper()
	ids, err := unixid.NewUnixID()
	if err != nil {
		t.Fatalf("unixid: %v", err)
	}
	return paymentmanager.New(orm.New(mem.New()), paymentmanager.Deps{
		IDs:      ids,
		TenantID: "t1",
		Charges: []paymentmanager.ChargeSource{
        func() paymentmanager.ChargeSource {
            if term == nil {
                return paymentmanager.ChargeSource{Module: refModule, Reader: fakeCharge{}}
            }
            return paymentmanager.ChargeSource{Module: refModule, Reader: fakeCharge{}, Device: "POS-1"}
        }(),
    },
		Terminal: term,
		Timeouts: timeouts,
	})
}

// payTerminal creates payment p1 by card (debit) as u1 through the loopback and answers the created payment.
func payTerminal(m *paymentmanager.Module, q *callQueue) (paymentmanager.Payment, error) {
	var out paymentmanager.Payment
	var callErr error
	crud.LoopbackActingAsWithQueue(nil, "u1", q, m).Call(
		router.Operation{Module: paymentmanager.ModelName, Entity: (&paymentmanager.Payment{}).ModelName(), Action: model.Create},
		&crud.CreateArgs{Records: []model.Model{&paymentmanager.Payment{
			Id: "p1", ReferenceModule: refModule, ReferenceId: "r1", Method: paymentmanager.MethodDebit, Channel: paymentmanager.ChannelTerminal, Currency: "CLP",
		}}},
		&out, func(err error) { callErr = err })
	return out, callErr
}

func TestTimeouts_OutOfRangeRefused(t *testing.T) {
	if _, err := newModule(t, sandbox.NewTerminal(), paymentmanager.Timeouts{}); err != nil {
		t.Fatalf("zero Timeouts must take the defaults: %v", err)
	}
	if _, err := newModule(t, sandbox.NewTerminal(), paymentmanager.Timeouts{TerminalPushSeconds: 11}); err == nil {
		t.Fatal("a push wait above the cap must refuse to start")
	}
}

func TestMount_MatchesDefinition(t *testing.T) {
	m, err := newModule(t, sandbox.NewTerminal(), paymentmanager.Timeouts{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := crud.MountWithQueue(&mock.Router{}, nil, nil, &callQueue{}, m); err != nil {
		t.Fatalf("the generated mount does not match the definition: %v", err)
	}
}

func TestTerminal_NoTerminalRefused(t *testing.T) {
	m, err := newModule(t, nil, paymentmanager.Timeouts{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := payTerminal(m, &callQueue{}); err == nil {
		t.Fatal("a card payment without a terminal must be refused")
	}
}

func TestTerminal_PushViaExternalCall(t *testing.T) {
	term := sandbox.NewTerminal()
	m, err := newModule(t, term, paymentmanager.Timeouts{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	q := &callQueue{}
	out, err := payTerminal(m, q)
	if err != nil {
		t.Fatalf("payTerminal: %v", err)
	}
	if term.Pushes() != 1 {
		t.Fatalf("expected one push to the machine, got %d", term.Pushes())
	}
	if out.ProviderToken != sandbox.TerminalTokenPrefix+"p1" {
		t.Fatalf("expected the machine's token, got %q", out.ProviderToken)
	}
	if out.Status != paymentmanager.StatusPending {
		t.Fatalf("a payment waiting for the card is pending, got %q", out.Status)
	}
	if len(q.added) != 2 || q.added[0].Call != "TerminalPush" || q.added[1].Call != "TerminalStatus" {
		t.Fatalf("expected TerminalPush then TerminalStatus queued, got %+v", q.added)
	}
}

func TestTerminal_Unreachable(t *testing.T) {
	term := sandbox.NewTerminal()
	term.FailNext(fmt.Err("machine offline"))
	m, err := newModule(t, term, paymentmanager.Timeouts{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	q := &callQueue{}
	out, err := payTerminal(m, q)
	if err != nil {
		t.Fatalf("the payment is created even when the machine does not answer: %v", err)
	}
	if out.Status != paymentmanager.StatusPending || out.ProviderToken != "" {
		t.Fatalf("expected pending with no token, got %q / %q", out.Status, out.ProviderToken)
	}
	if len(q.failed) == 0 {
		t.Fatal("the failed push must be recorded in the queue")
	}
}
