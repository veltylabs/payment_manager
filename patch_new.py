import re

with open("module.go", "r") as f:
    text = f.read()

text = text.replace('func New(db *orm.DB, deps Deps) (*Module, error) {\n\tif len(deps.Charges) == 0 {', '''func New(db *orm.DB, deps Deps) (*Module, error) {
\tif deps.IDs == nil {
\t\treturn nil, fmt.Err("payment_manager: Deps.IDs is required")
\t}
\tt, err := deps.Timeouts.withDefaults()
\tif err != nil {
\t\treturn nil, err
\t}
\tif len(deps.Charges) == 0 {''')

text = text.replace('return &Module{\n\t\tdb:       db,', '''return &Module{
\t\tdb:       db,
\t\tstore:    orm.TenantStore(db, deps.TenantID, deps.IDs),
\t\ttimeouts: t,''')

with open("module.go", "w") as f:
    f.write(text)
