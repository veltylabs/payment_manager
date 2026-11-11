import re

with open("ports.go", "r") as f:
    text = f.read()

# Delete OnlineResult
text = re.sub(r'type OnlineResult struct \{.*?\}\n', '', text, flags=re.DOTALL)

# Add OnlineReturn
text += '''
// OnlineReturn identifies the payment a provider's return refers to. It carries no outcome: the module asks the gateway.
type OnlineReturn struct {
	PaymentId string // the buy order the provider echoes, when it sends one
	Token     string // the provider token of the transaction, when it sends one
}
'''

# Add Confirm to Gateway
text = re.sub(r'type Gateway interface \{.*?Refund\(token string, amount int64\) \(GatewayResult, error\)', r'\g<0>\n\tConfirm(token string) (GatewayResult, error) // finalizes the transaction when the provider requires it and reports its real state; idempotent', text, flags=re.DOTALL)

with open("ports.go", "w") as f:
    f.write(text)
