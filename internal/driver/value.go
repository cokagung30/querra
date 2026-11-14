package driver

import (
	"database/sql"
	"fmt"
)

// Kind is how the UI treats a column's values: alignment, rendering and the
// cell editor (PLAN §3.5).
type Kind string

const (
	KindText     Kind = "text"
	KindInteger  Kind = "integer"
	KindDecimal  Kind = "decimal" // exact and floating-point numbers
	KindBool     Kind = "bool"
	KindDateTime Kind = "datetime"
	KindDate     Kind = "date"
	KindTime     Kind = "time"
	KindJSON     Kind = "json"
	KindUUID     Kind = "uuid"
	KindBinary   Kind = "binary"
	KindArray    Kind = "array"
	KindOther    Kind = "other"
)

// ResultColumn describes one column of a query result.
type ResultColumn struct {
	Name string `json:"name"`
	// DatabaseType is the engine's name for the type, e.g. "int8".
	DatabaseType string `json:"databaseType"`
	Kind         Kind   `json:"kind"`
}

// Codec maps an engine's result columns to kinds and its values to cell
// text. Cells always cross to the frontend as text, so 64-bit integers and
// decimals keep every digit (PLAN §3.5).
type Codec interface {
	Column(ct *sql.ColumnType) ResultColumn
	// Format returns the text of a non-NULL value of col, as database/sql
	// scanned it into an any.
	Format(col ResultColumn, v any) string
}

// Columns describes the columns of rows.
func Columns(rows *sql.Rows, codec Codec) ([]ResultColumn, error) {
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("read result columns: %w", err)
	}
	cols := make([]ResultColumn, len(types))
	for i, ct := range types {
		cols[i] = codec.Column(ct)
	}
	return cols, nil
}

// ScanRow reads the row rows.Next moved to: nil for NULL, the codec's text
// otherwise. cols comes from Columns.
func ScanRow(rows *sql.Rows, codec Codec, cols []ResultColumn) ([]*string, error) {
	values := make([]any, len(cols))
	dest := make([]any, len(cols))
	for i := range values {
		dest[i] = &values[i]
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, fmt.Errorf("scan row: %w", err)
	}
	cells := make([]*string, len(cols))
	for i, v := range values {
		if v != nil {
			s := codec.Format(cols[i], v)
			cells[i] = &s
		}
	}
	return cells, nil
}
