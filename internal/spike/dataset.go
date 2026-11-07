// Package spike holds temporary code for the M0.2 stack validation (results in
// docs/spikes/fase-0.md). It is deleted once the spike's decisions are made;
// nothing outside internal/app may depend on it.
package spike

import (
	"fmt"
	"runtime"
	"strconv"
	"time"
)

// Column describes one generated column.
type Column struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// Dataset is an in-memory result set shaped like the planned resultset buffer:
// every cell is a *string, and nil means NULL.
type Dataset struct {
	Columns []Column
	rows    [][]*string
}

var columnKinds = []string{"integer", "text", "decimal", "datetime", "bool", "text", "uuid", "json", "integer", "text"}

// NewDataset generates rowCount×colCount cells deterministically.
func NewDataset(rowCount, colCount int) *Dataset {
	cols := make([]Column, colCount)
	for c := range cols {
		kind := columnKinds[c%len(columnKinds)]
		cols[c] = Column{Name: fmt.Sprintf("col_%02d_%s", c+1, kind), Kind: kind}
	}

	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	rows := make([][]*string, rowCount)
	for r := range rows {
		row := make([]*string, colCount)
		for c := range row {
			if (r+c)%17 == 0 {
				continue // leave some cells NULL
			}
			v := cellValue(cols[c].Kind, r, c, base)
			row[c] = &v
		}
		rows[r] = row
	}
	return &Dataset{Columns: cols, rows: rows}
}

func cellValue(kind string, r, c int, base time.Time) string {
	switch kind {
	case "integer":
		return strconv.Itoa(r*31 + c)
	case "decimal":
		return strconv.FormatFloat(float64(r%100000)/100+float64(c), 'f', 2, 64)
	case "datetime":
		return base.Add(time.Duration(r) * time.Minute).Format(time.RFC3339)
	case "bool":
		return strconv.FormatBool((r+c)%2 == 0)
	case "uuid":
		return fmt.Sprintf("%08x-0000-4000-8000-%012x", r, c)
	case "json":
		return fmt.Sprintf(`{"row":%d,"col":%d,"tags":["a","b"]}`, r, c)
	default:
		return fmt.Sprintf("row %d text value for column %d", r, c)
	}
}

// RowCount returns the number of rows.
func (d *Dataset) RowCount() int {
	return len(d.rows)
}

// Page returns up to limit rows starting at offset. Out-of-range requests
// return an empty page.
func (d *Dataset) Page(offset, limit int) [][]*string {
	offset = max(offset, 0)
	if offset >= len(d.rows) || limit <= 0 {
		return [][]*string{}
	}
	return d.rows[offset:min(offset+limit, len(d.rows))]
}

// MemStats is the Go side's memory use. The webview's memory is separate and
// is measured from outside the process (dev/perf).
type MemStats struct {
	HeapAllocMB float64 `json:"heapAllocMB"`
	SysMB       float64 `json:"sysMB"`
}

// ReadMemStats returns current Go memory use.
func ReadMemStats() MemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	const mb = 1 << 20
	return MemStats{HeapAllocMB: float64(m.HeapAlloc) / mb, SysMB: float64(m.Sys) / mb}
}
