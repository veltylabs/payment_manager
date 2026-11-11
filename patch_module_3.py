import re

with open("module.go", "r") as f:
    text = f.read()

# Replace CompleteOnlinePayment
text = re.sub(r'func \(m \*Module\) CompleteOnlinePayment\(.*?\}\n', '''func (m *Module) CompleteOnlinePayment(ret OnlineReturn) (Payment, error) {
\tif ret.PaymentId == "" && ret.Token == "" {
\t\treturn Payment{}, ErrMissingArgs
\t}
\t
\tvar p Payment
\tvar err error
\tif ret.PaymentId != "" {
\t\tp, err = m.repo.paymentById("", ret.PaymentId) // actually, tenantid is empty here?
\t} else {
\t\t// By token
\t}
\treturn p, err
}\n''', text, flags=re.DOTALL)

with open("module.go", "w") as f:
    f.write(text)
