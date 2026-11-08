package app

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/cokagung30/querra/internal/perf"
)

func TestPerfServiceInertWithoutAutorun(t *testing.T) {
	t.Setenv("QUERRA_PERF_AUTORUN", "")
	s := NewPerfService(time.Now())

	if mode := s.Ready(); mode != "" {
		t.Fatalf("Ready() = %q, want empty mode", mode)
	}
	if info := s.OpenDataset(); info.RowCount != 0 {
		t.Fatalf("OpenDataset() outside grid mode generated %d rows", info.RowCount)
	}
	if rows := s.FetchRows(0, 10); len(rows) != 0 {
		t.Fatalf("FetchRows() without a dataset returned %d rows", len(rows))
	}
	// Report and Finish must not panic or quit without an autorun.
	s.Report("grid", map[string]any{"x": 1})
	s.Finish()
}

func TestPerfServiceIgnoresUnknownMode(t *testing.T) {
	t.Setenv("QUERRA_PERF_AUTORUN", "cancel")
	if mode := NewPerfService(time.Now()).Ready(); mode != "" {
		t.Fatalf("Ready() = %q for an unknown mode, want empty", mode)
	}
}

func TestPerfServiceGridMode(t *testing.T) {
	var buf bytes.Buffer
	s := &PerfService{processStart: time.Now(), autorun: perfModeGrid, report: perf.NewReporterTo(&buf)}

	if mode := s.Ready(); mode != perfModeGrid {
		t.Fatalf("Ready() = %q, want %q", mode, perfModeGrid)
	}
	if !strings.Contains(buf.String(), `"event":"ready"`) {
		t.Fatalf("Ready() did not report: %q", buf.String())
	}

	info := s.OpenDataset()
	if info.RowCount != perfRows || len(info.Columns) != perfColumns {
		t.Fatalf("OpenDataset() = %d rows × %d columns, want %d × %d",
			info.RowCount, len(info.Columns), perfRows, perfColumns)
	}
	if rows := s.FetchRows(0, 5000); len(rows) != maxPerfFetchRows {
		t.Fatalf("FetchRows(0, 5000) returned %d rows, want the %d-row cap", len(rows), maxPerfFetchRows)
	}
}
