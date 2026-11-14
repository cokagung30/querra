package driver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"testing"

	_ "modernc.org/sqlite" // a real *sql.Rows for ScanRow
)

type fakeDriver struct {
	Driver // methods other than Engine are never called here
	engine Engine
}

func (d fakeDriver) Engine() Engine { return d.engine }

func TestRegistry(t *testing.T) {
	const engine Engine = "test-registry"
	if _, err := Lookup(engine); !errors.Is(err, ErrUnknownEngine) {
		t.Fatalf("Lookup() before Register error = %v, want ErrUnknownEngine", err)
	}

	Register(fakeDriver{engine: engine})
	d, err := Lookup(engine)
	if err != nil || d.Engine() != engine {
		t.Fatalf("Lookup() = %v, %v; want the registered driver", d, err)
	}
	if !slices.Contains(Engines(), engine) {
		t.Fatalf("Engines() = %v, want it to include %q", Engines(), engine)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("registering an engine twice did not panic")
		}
	}()
	Register(fakeDriver{engine: engine})
}

// sprintCodec formats values with fmt, enough to check ScanRow's plumbing.
type sprintCodec struct{}

func (sprintCodec) Column(ct *sql.ColumnType) ResultColumn {
	return ResultColumn{Name: ct.Name(), DatabaseType: ct.DatabaseTypeName(), Kind: KindOther}
}

func (sprintCodec) Format(_ ResultColumn, v any) string {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return fmt.Sprint(v)
}

func TestScanRowKeepsNullApartFromEmptyText(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	rows, err := db.QueryContext(context.Background(), `SELECT 42 AS n, NULL AS missing, '' AS empty, 'x' AS s`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()

	cols, err := Columns(rows, sprintCodec{})
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{cols[0].Name, cols[1].Name, cols[2].Name, cols[3].Name}; !slices.Equal(got, []string{"n", "missing", "empty", "s"}) {
		t.Fatalf("column names = %v", got)
	}
	if !rows.Next() {
		t.Fatal("no row")
	}
	cells, err := ScanRow(rows, sprintCodec{}, cols)
	if err != nil {
		t.Fatal(err)
	}
	if cells[1] != nil {
		t.Errorf("NULL cell = %q, want nil", *cells[1])
	}
	for i, want := range map[int]string{0: "42", 2: "", 3: "x"} {
		if cells[i] == nil || *cells[i] != want {
			t.Errorf("cell %d = %v, want %q", i, cells[i], want)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
