← [Index](PLAN.md) | Next → [Stage 2](PLAN_STAGE_2_module_go.md)

> Read AGENTS.md and the index `PLAN.md` (rules, tables, external calls) first. Commit when this stage compiles.

## Stage 1 — `definition.go`

- Set `Allow` on each table exactly as in «Tables, stores and rules»; a table not listed there gets no `Allow` (internal).
- Mark `ServerOnly: true`: `tenant_id`, `created_at`, `updated_at`, and every field the server fills (amounts, `status`, `failure`, `provider`, tokens, `redirect_url`, `payment_id`).
- Delete every `*Args` / `*Result` definition that only carried a removed operation (`grep -n "ArgsModel\|ResultModel" model.go` → only the ones this plan keeps).
- New table `OnlinePaymentRequestModel` (name `payment_online_request`): `id`, `tenant_id`, `reference_module`, `reference_id`, `return_url` (`input.Text()`), `payment_id`, `redirect_url`, `status`, `failure`, `created_at` (server fields `ServerOnly`).
- New table `PaymentReconciliationModel` (name `payment_reconciliation`): `id`, `tenant_id`, `status`, `failure`, `reconciled` (`model.Int()`), `actor_id`, `created_at`; `Allow: model.Create | model.Read`.
- Declare `ExternalCalls: []model.ExternalCallDefinition{...}` on:
  - `PaymentModel`:
    - `{Name: "TerminalPush", Mode: model.WaitForReply, MaxWaitSeconds: TerminalPushCapSeconds}`
    - `{Name: "TerminalStatus", Mode: model.RetryInBackground, MaxAttempts: 20}`
  - `PaymentManualConfirmationModel`:
    - `{Name: "Confirm", Mode: model.WaitForReply, MaxWaitSeconds: 10}`
  - `OnlinePaymentRequestModel`:
    - `{Name: "GatewayStart", Mode: model.WaitForReply, MaxWaitSeconds: OnlineStartCapSeconds}`
  - `RefundModel`:
    - `{Name: "GatewayRefund", Mode: model.WaitForReply, MaxWaitSeconds: 15}`
  - `PaymentReconciliationModel`:
    - `{Name: "Reconcile", Mode: model.RetryInBackground, MaxAttempts: 3}`
- New table `PaymentManualConfirmationModel` (name `payment_manual_confirmation`): `id`, `tenant_id`, `payment_id`, `note` (`input.Text()`), `actor_id`, `created_at` (`actor_id`, `created_at` `ServerOnly`). `OnlinePaymentRequestModel` also carries `payer_name`, `payer_email`, `payer_phone` (client fields) and `token`, `token_field` (`ServerOnly`).
- Add the three new tables to `migrate/`. `ResourcePayment`, `ResourcePaymentOnline`, `ResourceRefund` are deleted (each table/read model is its resource).
- **Keep every `*Args` model that an exported reader's signature still uses** (`grep -n "^func (m \*Module) [A-Z][A-Za-z]*([^)]*Args" *.go` lists them): mark each `Wire: true`; delete only the `*Args` no remaining signature uses.
- D52 (the entity is its own filter): a filter model (`*Filter`, `*FilterModel`, an `*ArgsModel` that only carried filter values) is not kept: its fields become fields of the read model or table (`OmitEmpty`), and the rule reads them from its receiver; delete the filter type and its `Wire`/`NewFilter` plumbing.
- Add `var Definition = model.ModuleDefinition{Name: "payment_manager", Entities: []model.Definition{…every table and read model of this plan…}}` and, on every table and read model, `Rules` as «Tables, stores and rules» says (`AddMyBusinessRule` where a rule is named, `HaveNoBusinessRule` for the other allowed actions; a read model: `Read: model.AddMyBusinessRule`). Delete the hand topics (`Topic*`) and `Err*`/resources no code uses any more.
