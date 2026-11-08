package perf

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDatasetPage(t *testing.T) {
	d := NewDataset(1000, 20)

	tests := []struct {
		name          string
		offset, limit int
		wantLen       int
	}{
		{"first page", 0, 500, 500},
		{"last partial page", 900, 500, 100},
		{"past the end", 1000, 500, 0},
		{"negative offset", -10, 5, 5},
		{"zero limit", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(d.Page(tt.offset, tt.limit)); got != tt.wantLen {
				t.Fatalf("Page(%d, %d) returned %d rows, want %d", tt.offset, tt.limit, got, tt.wantLen)
			}
		})
	}
}

func TestDatasetHasNulls(t *testing.T) {
	d := NewDataset(1, 20)
	if d.Page(0, 1)[0][0] != nil {
		t.Fatal("cell (0,0) should be NULL")
	}
	if d.Page(0, 1)[0][1] == nil {
		t.Fatal("cell (0,1) should not be NULL")
	}
}

func TestReporterEmit(t *testing.T) {
	var buf bytes.Buffer
	NewReporterTo(&buf).Emit("ready", map[string]any{"ms": 42})

	line, ok := strings.CutPrefix(strings.TrimSpace(buf.String()), ReportPrefix)
	if !ok {
		t.Fatalf("line %q lacks prefix %q", buf.String(), ReportPrefix)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("line is not JSON: %v", err)
	}
	if got["event"] != "ready" || got["ms"] != float64(42) || got["unixMs"] == nil {
		t.Fatalf("unexpected report line: %v", got)
	}
}
