import re

with open("module.go", "r") as f:
    text = f.read()

# Replace `CompleteOnlinePayment(r OnlineResult)` with `CompleteOnlinePayment(ret OnlineReturn)`
text = re.sub(r'func \(m \*Module\) CompleteOnlinePayment\(.*?\n.*?(?=func \(m \*Module\))', '''func (m *Module) CompleteOnlinePayment(ret OnlineReturn) (Payment, error) {
\tif ret.PaymentId == "" && ret.Token == "" {
\t\treturn Payment{}, ErrMissingArgs
\t}
\t
\tvar p Payment
\tvar err error
\tif ret.PaymentId != "" {
\t\tp, err = m.repo.paymentById("", ret.PaymentId) // Actually, need tenant ID? Let's assume we can fetch it, wait repo needs tenantId? Or we do a query without it if it's unique...
\t\t// Wait, OnlineReturn doesn't have TenantId. How to fetch it?
\t}
}\n\n''', text, flags=re.DOTALL)

with open("module.go", "w") as f:
    f.write(text)
