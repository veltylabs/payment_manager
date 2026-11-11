import re

with open("rule_payment_status_result_read.go", "r") as f:
    text = f.read()

text = text.replace('filter.TenantId', 'args.TenantID')
with open("rule_payment_status_result_read.go", "w") as f:
    f.write(text)

with open("rule_charge_result_read.go", "r") as f:
    text = f.read()

text = text.replace('filter.TenantId', 'args.TenantID')
with open("rule_charge_result_read.go", "w") as f:
    f.write(text)

with open("view.go", "r") as f:
    text = f.read()

text = re.sub(r'func NewView\(caller router\.Caller\) \*View \{.*?\n\}', '''func NewView(caller router.Caller) *View {
\treturn &View{
\t\tList: view.NewCallerLister(caller, view.Ops{Module: "payment_manager", Entity: "payment", Read: true}, func() model.TableSlice { return &PaymentList{} }),
\t}
}''', text, flags=re.DOTALL)

with open("view.go", "w") as f:
    f.write(text)
