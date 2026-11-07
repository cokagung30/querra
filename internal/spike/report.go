package spike

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// ReportPrefix starts every report line so scripts can pick them out of other
// output.
const ReportPrefix = "SPIKE "

// Reporter writes one JSON object per line for the dev/perf scripts: to the
// file named by QUERRA_SPIKE_REPORT, or to stdout when that is unset (Windows
// GUI builds have no stdout, so they need the file).
type Reporter struct {
	mu sync.Mutex
	w  io.Writer
}

// NewReporter returns a Reporter for the current environment.
func NewReporter() *Reporter {
	if path := os.Getenv("QUERRA_SPIKE_REPORT"); path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err == nil {
			return &Reporter{w: f}
		}
		fmt.Fprintf(os.Stderr, "spike: open report file: %v\n", err)
	}
	return &Reporter{w: os.Stdout}
}

// NewReporterTo returns a Reporter writing to w.
func NewReporterTo(w io.Writer) *Reporter {
	return &Reporter{w: w}
}

// Emit writes event with its fields and a unixMs timestamp.
func (r *Reporter) Emit(event string, fields map[string]any) {
	line := map[string]any{"event": event, "unixMs": time.Now().UnixMilli()}
	for k, v := range fields {
		line[k] = v
	}
	data, err := json.Marshal(line)
	if err != nil {
		data = []byte(fmt.Sprintf(`{"event":"error","err":%q}`, err.Error()))
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.w, "%s%s\n", ReportPrefix, data)
}
