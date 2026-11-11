---
PLAN: "test: payment_manager rule tests, part 2a (queue double and five tests)"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work. Commit after every stage.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
> **Publishing:** you publish only by OPENING THE PULL REQUEST (a `git push` from your sandbox does not reach GitHub). Submit after EACH test file that compiles and keep committing.
> **Never ask a yes/no question about progress**; ask only when a signature in this plan is wrong, and name the symbol.
> **Before you submit:** `git status` must be clean and `git diff --stat main` must list the files of the plan. A pull request that carries only go.mod, go.sum or docs/PLAN.md is a failed delivery: `git add -A`, commit and submit again.

# Plan: `payment_manager` — the first five tests (part 2a)

The module is migrated (v0.4.47) and its `CompleteOnlinePayment` now asks the gateway (`Gateway.Confirm`). Its old tests were deleted with the removed code; this plan writes the first five, small and exact, with the sandbox fakes of the repo (`sandbox.NewTerminal()`, `sandbox.New(url)`). Nothing else: no translations, no docs (part 2b). Never edit `definition_gen.go`.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```
No `go get`, no `replace`. Read files by ranges (`sed -n 1,80p file`).

## Stage 1 — `tests/queue_double_test.go` (new, copy as is)

```go
package tests

import (
	"webtyp.com/crud"
	"webtyp.com/fmt"
	"webtyp.com/model"
)

// callQueue is the test double of crud.CallQueue: it records what crud asks and stores nothing.
type callQueue struct {
	added  []crud.CallRequest
	done   []string
	failed []string // the reasons
}

func (q *callQueue) Add(tx crud.Tx, r crud.CallRequest) (string, error) {
	q.added = append(q.added, r)
	return fmt.Sprintf("call-%d", len(q.added)), nil
}
func (q *callQueue) MarkDone(id string) error { q.done = append(q.done, id); return nil }
func (q *callQueue) MarkFailedAttempt(id string, call model.ExternalCallDefinition, reason string) error {
	q.failed = append(q.failed, reason)
	return nil
}
func (q *callQueue) ConfirmCall(id string) error        { return nil }
func (q *callQueue) RejectCall(id, reason string) error { return nil }
```

## Stage 2 — `tests/rules_test.go` (new), written exactly (adapt only a name the compiler refuses, and say which in the PR)

```go
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
		Charges:  []paymentmanager.ChargeSource{{Module: refModule, Reader: fakeCharge{}, Device: "POS-1"}},
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
			Id: "p1", ReferenceModule: refModule, ReferenceId: "r1", Method: paymentmanager.MethodDebit,
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
```
(`unixid.NewUnixID()` is the real id generator used by `seed/demo.go`; `fmt.Err` is `webtyp.com/fmt`.) Each test has its row in `docs/TESTS.md` (`| [Name](../tests/rules_test.go) | rule |`, header `| Test | Rule |`).

## Delivery check

`gotest` passes on every destination; the five tests exist with their rows. Submit as soon as Stage 1 and the first test pass, then keep committing. A tree that does not compile or whose tests did not run is a failed delivery.
