import re

with open("definition.go", "r") as f:
    content = f.read()

content = content.replace(
    '{Name: "tenant_id", Type: model.Text(), NotNull: true},',
    '{Name: "tenant_id", Type: model.Text(), NotNull: true, ServerOnly: true},'
)
content = content.replace(
    '{Name: "created_at", Type: model.UnixTime()},',
    '{Name: "created_at", Type: model.UnixTime(), ServerOnly: true},'
)
content = content.replace(
    '{Name: "updated_at", Type: model.UnixTime()},',
    '{Name: "updated_at", Type: model.UnixTime(), ServerOnly: true},'
)

# payment_id is also ServerOnly. amount, status, failure, provider, tokens, redirect_url, payment_id
content = content.replace(
    '{Name: "status", Type: model.Text(), NotNull: true, Item: model.ItemDescription},',
    '{Name: "status", Type: model.Text(), NotNull: true, Item: model.ItemDescription, ServerOnly: true},'
)
content = content.replace(
    '{Name: "amount", Type: model.Int(), NotNull: true, Item: model.ItemLabel},',
    '{Name: "amount", Type: model.Int(), NotNull: true, Item: model.ItemLabel, ServerOnly: true},'
)
content = content.replace(
    '{Name: "provider_token", Type: model.Text(), OmitEmpty: true},',
    '{Name: "provider_token", Type: model.Text(), OmitEmpty: true, ServerOnly: true},'
)
content = content.replace(
    "{Name: \"provider\", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'_', '-'}, Maximum: 30}},",
    "{Name: \"provider\", Type: model.Text(), OmitEmpty: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'_', '-'}, Maximum: 30}, ServerOnly: true},"
)

with open("definition.go", "w") as f:
    f.write(content)
