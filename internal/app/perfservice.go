package app

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/cokagung30/querra/internal/spike"
)

func init() {
	application.RegisterEvent[spike.CancelResult]("spike:cancel-result")
}

const (
	spikeRows    = 100_000
	spikeColumns = 20
)

// SpikeService exposes the M0.2 spike probes to the frontend. Temporary: it is
// removed together with internal/spike.
//
// QUERRA_SPIKE_AUTORUN selects an unattended benchmark for the dev/perf
// scripts ("startup", "idle", "grid", "cancel", "keychain-write",
// "keychain-read"); empty means interactive use.
type SpikeService struct {
	processStart time.Time
	autorun      string
	report       *spike.Reporter

	mu      sync.Mutex
	dataset *spike.Dataset
}

// DatasetInfo describes the generated spike dataset.
type DatasetInfo struct {
	Columns    []spike.Column `json:"columns"`
	RowCount   int            `json:"rowCount"`
	GenerateMs int64          `json:"generateMs"`
}

// NewSpikeService returns a SpikeService that measures startup from processStart.
func NewSpikeService(processStart time.Time) *SpikeService {
	return &SpikeService{
		processStart: processStart,
		autorun:      os.Getenv("QUERRA_SPIKE_AUTORUN"),
		report:       spike.NewReporter(),
	}
}

// AutorunMode returns the benchmark the frontend should run unattended, or ""
// for interactive use.
func (s *SpikeService) AutorunMode() string {
	return s.autorun
}

// ReportReady records how long the app took to show its first screen. In
// "startup" mode it then quits.
func (s *SpikeService) ReportReady() {
	s.report.Emit("ready", map[string]any{
		"sinceProcessStartMs": time.Since(s.processStart).Milliseconds(),
		"mem":                 spike.ReadMemStats(),
	})
	if s.autorun == "startup" {
		s.quitAfter(0)
	}
}

// OpenDataset generates the 100k×20 dataset on first use and describes it.
func (s *SpikeService) OpenDataset() DatasetInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	var generateMs int64
	if s.dataset == nil {
		start := time.Now()
		s.dataset = spike.NewDataset(spikeRows, spikeColumns)
		generateMs = time.Since(start).Milliseconds()
	}
	return DatasetInfo{Columns: s.dataset.Columns, RowCount: s.dataset.RowCount(), GenerateMs: generateMs}
}

// FetchRows returns one window of the dataset; nil cells are NULL.
func (s *SpikeService) FetchRows(offset, limit int) [][]*string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dataset == nil {
		return [][]*string{}
	}
	return s.dataset.Page(offset, limit)
}

// MemStats returns the Go side's current memory use.
func (s *SpikeService) MemStats() spike.MemStats {
	return spike.ReadMemStats()
}

// SleepAndObserve runs a 30-second server-side sleep on engine ("postgres" or
// "mysql") until the frontend cancels the call. A cancelled promise discards
// its result, so the outcome also goes out as the "spike:cancel-result" event.
func (s *SpikeService) SleepAndObserve(ctx context.Context, engine string) spike.CancelResult {
	res := spike.SleepAndObserve(ctx, engine)
	s.report.Emit("cancel", map[string]any{"result": res})
	application.Get().Event.Emit("spike:cancel-result", res)
	return res
}

// KeychainWrite stores a dummy value in the spike's keychain entry.
func (s *SpikeService) KeychainWrite(value string) spike.KeychainStep {
	return s.reportKeychain(spike.KeychainWrite(value))
}

// KeychainRead reads the spike's keychain entry.
func (s *SpikeService) KeychainRead() spike.KeychainStep {
	return s.reportKeychain(spike.KeychainRead())
}

// KeychainDelete removes the spike's keychain entry.
func (s *SpikeService) KeychainDelete() spike.KeychainStep {
	return s.reportKeychain(spike.KeychainDelete())
}

func (s *SpikeService) reportKeychain(step spike.KeychainStep) spike.KeychainStep {
	s.report.Emit("keychain", map[string]any{"step": step})
	return step
}

// Report records a result measured in the frontend, such as a scroll benchmark.
func (s *SpikeService) Report(event string, data map[string]any) {
	s.report.Emit(event, data)
}

// Finish ends an autorun: it reports memory use, then quits after
// QUERRA_SPIKE_LINGER_MS (default 3000) so the dev/perf scripts can sample the
// process from outside first.
func (s *SpikeService) Finish() {
	s.report.Emit("finish", map[string]any{"mem": spike.ReadMemStats()})
	linger := 3000
	if v, err := strconv.Atoi(os.Getenv("QUERRA_SPIKE_LINGER_MS")); err == nil {
		linger = v
	}
	s.quitAfter(time.Duration(linger) * time.Millisecond)
}

func (s *SpikeService) quitAfter(d time.Duration) {
	go func() {
		time.Sleep(d)
		application.Get().Quit()
	}()
}
