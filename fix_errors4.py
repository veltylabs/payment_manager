import re

with open("rule_payment_status_result_read.go", "r") as f:
    text = f.read()

text = text.replace('args.TenantID', 'a.TenantID')
with open("rule_payment_status_result_read.go", "w") as f:
    f.write(text)

with open("rule_charge_result_read.go", "r") as f:
    text = f.read()

text = text.replace('args.TenantID', 'a.TenantID')
with open("rule_charge_result_read.go", "w") as f:
    f.write(text)

with open("view.go", "r") as f:
    text = f.read()

text = re.sub(r'func NewView\(caller router\.Caller, opts \.\.\.view\.Option\) view\.Presenter \{.*?\n\}', '''func NewView(caller router.Caller, opts ...view.Option) view.Presenter {
\tallOpts := append([]view.Option{view.WithTitle(titlePayments)}, opts...)
\tlister := view.NewCallerLister(caller, view.Ops{Module: "payment_manager", Entity: "payment", Read: true, Page: view.Page{First: 50, Size: 50, More: true}}, func() model.TableSlice { return &PaymentList{} })
\treturn view.New(lister, &Payment{}, allOpts...)
}''', text, flags=re.DOTALL)

with open("view.go", "w") as f:
    f.write(text)

with open("module.go", "r") as f:
    text = f.read()

text = text.replace('func (m *Module) CompleteOnlinePayment(ret OnlineResult) (Payment, error) {', 'func (m *Module) CompleteOnlinePayment(ret OnlineReturn) (Payment, error) {')
with open("module.go", "w") as f:
    f.write(text)
