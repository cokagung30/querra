package app

import (
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/cokagung30/querra/internal/perf"
)

// Autorun modes that dev/perf selects with QUERRA_PERF_AUTORUN.
const (
	perfModeStartup = "startup" // report the first painted screen, then quit
	perfModeIdle    = "idle"    // report the first painted screen, then stay open
	perfModeGrid    = "grid"    // scroll benchmark over a generated dataset
)

const (
	perfRows    = 100_000
	perfColumns = 20
	// maxPerfFetchRows bounds one FetchRows window.
	maxPerfFetchRows = 1000
)

// PerfService drives the unattended measurements in dev/perf (PLAN §6): startup
// time, idle memory, and a grid scroll benchmark. Without QUERRA_PERF_AUTORUN it
// does nothing, so it is safe to ship in release builds, which is what dev/perf
// measures.
type PerfService struct {
	processStart time.Time
	autorun      string
	report       *perf.Reporter

	mu      sync.Mutex
	dataset *perf.Dataset
}

// DatasetInfo describes the generated benchmark dataset.
type DatasetInfo struct {
	Columns    []perf.Column `json:"columns"`
	RowCount   int           `json:"rowCount"`
	GenerateMs int64         `json:"generateMs"`
}

// NewPerfService returns a PerfService that measures startup from processStart.
func NewPerfService(processStart time.Time) *PerfService {
	s := &PerfService{processStart: processStart}
	switch mode := os.Getenv("QUERRA_PERF_AUTORUN"); mode {
	case "":
	case perfModeStartup, perfModeIdle, perfModeGrid:
		s.autorun = mode
		s.report = perf.NewReporter()
	default:
		slog.Warn("perf: unknown QUERRA_PERF_AUTORUN mode, ignoring it", "mode", mode)
	}
	return s
}

func (s *PerfService) measuring() bool {
	return s.autorun != ""
}

// Ready is called once the shell's first frame is painted. It returns the
// autorun mode the frontend should run ("" for normal use). While measuring it
// records the startup time, and in "startup" mode it then quits.
func (s *PerfService) Ready() string {
	if !s.measuring() {
		return ""
	}
	s.report.Emit("ready", map[string]any{
		"sinceProcessStartMs": time.Since(s.processStart).Milliseconds(),
		"mem":                 perf.ReadMemStats(),
	})
	if s.autorun == perfModeStartup {
		s.quitAfter(0)
	}
	return s.autorun
}

// OpenDataset generates the 100k×20 benchmark dataset on first use and
// describes it. Outside "grid" mode it returns an empty dataset, so nothing
// allocates it by accident.
func (s *PerfService) OpenDataset() DatasetInfo {
	if s.autorun != perfModeGrid {
		return DatasetInfo{Columns: []perf.Column{}}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var generateMs int64
	if s.dataset == nil {
		start := time.Now()
		s.dataset = perf.NewDataset(perfRows, perfColumns)
		generateMs = time.Since(start).Milliseconds()
	}
	return DatasetInfo{Columns: s.dataset.Columns, RowCount: s.dataset.RowCount(), GenerateMs: generateMs}
}

// FetchRows returns one window of at most 1000 rows of the dataset; nil cells
// are NULL.
func (s *PerfService) FetchRows(offset, limit int) [][]*string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dataset == nil {
		return [][]*string{}
	}
	return s.dataset.Page(offset, min(limit, maxPerfFetchRows))
}

// Report records a result measured in the frontend, such as the scroll
// benchmark. It is ignored unless an autorun is active.
func (s *PerfService) Report(event string, data map[string]any) {
	if s.measuring() {
		s.report.Emit(event, data)
	}
}

// Finish ends an autorun: it reports memory use, then quits after
// QUERRA_PERF_LINGER_MS (default 3000) so dev/perf can sample the process
// from outside first. It is ignored unless an autorun is active.
func (s *PerfService) Finish() {
	if !s.measuring() {
		return
	}
	s.report.Emit("finish", map[string]any{"mem": perf.ReadMemStats()})
	linger := 3000
	if v, err := strconv.Atoi(os.Getenv("QUERRA_PERF_LINGER_MS")); err == nil {
		linger = v
	}
	s.quitAfter(time.Duration(linger) * time.Millisecond)
}

func (s *PerfService) quitAfter(d time.Duration) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("perf: quit panicked", "panic", r)
			}
		}()
		time.Sleep(d)
		application.Get().Quit()
	}()
}
