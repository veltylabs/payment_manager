package tests

import (
	"testing"

	"webtyp.com/ddl"
	"webtyp.com/model"

	pm "github.com/veltylabs/payment_manager"
	"github.com/veltylabs/payment_manager/migrate"
)

type recordingExecer struct{ queries []string }

func (r *recordingExecer) Exec(query string, args ...any) error {
	r.queries = append(r.queries, query)
	return nil
}

type tableCompiler struct{}

func (tableCompiler) CompileDDL(stmt ddl.Stmt, m model.Model) (string, []any, error) {
	return stmt.Table, nil, nil
}

func TestMigrate_SyncsTables(t *testing.T) {
	ex := &recordingExecer{}
	if err := migrate.Migrate(ex, tableCompiler{}); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	var tables []string
	for _, q := range ex.queries {
		if len(tables) == 0 || tables[len(tables)-1] != q {
			tables = append(tables, q)
		}
	}
	if len(tables) != 3 || tables[0] != pm.PaymentModel.Name || tables[1] != pm.RefundModel.Name || tables[2] != pm.PaymentCounterModel.Name {
		t.Fatalf("tables = %v, want payment, refund, payment_counter", tables)
	}
}
