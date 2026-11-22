package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/cokagung30/querra/internal/app"
	"github.com/cokagung30/querra/internal/apperr"
	_ "github.com/cokagung30/querra/internal/driver/postgres" // registers the PostgreSQL driver
	"github.com/cokagung30/querra/internal/secrets"
	"github.com/cokagung30/querra/internal/store"
)

// version is overridden for release builds with -ldflags "-X main.version=<version>".
var version = "0.1.0-dev"

// processStart is taken as early as Go allows, for the startup measurement
// in dev/perf (PerfService).
var processStart = time.Now()

// assets holds the built frontend (frontend/dist), served to the webview.
//
//go:embed all:frontend/dist
var assets embed.FS

// serviceOptions applies to every service: errors reach the frontend as an
// *apperr.Error in error.cause. In Wails v3.1.0 Options.MarshalError is never
// called, so each service needs it in its own options.
var serviceOptions = application.ServiceOptions{MarshalError: apperr.Marshal}

func service[T any](instance *T) application.Service {
	return application.NewServiceWithOptions(instance, serviceOptions)
}

func main() {
	services := []application.Service{
		service(app.NewAppService(version)),
		service(app.NewPerfService(processStart)),
	}
	st, storePath, storeErr := openStore()
	if storeErr == nil {
		services = append(services, service(app.NewConnectionService(st, secrets.NewKeyring())))
	} else {
		slog.Error("open local store", "path", storePath, "err", storeErr)
	}

	wailsApp := application.New(application.Options{
		Name:         "Querra",
		Description:  "Lightweight database client",
		Services:     services,
		MarshalError: apperr.Marshal, // what Wails documents; see serviceOptions
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	if runtime.GOOS == "darwin" {
		wailsApp.Menu.Set(app.MacMenu())
	}

	if storeErr != nil {
		// Without its store Querra has nothing to show. Say why and quit,
		// rather than open a window that cannot work.
		wailsApp.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
			wailsApp.Dialog.Error().
				SetTitle("Querra can't open its data").
				SetMessage(storeErrorMessage(storePath, storeErr)).
				Show()
			wailsApp.Quit()
		})
	} else {
		wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:     "Querra",
			Width:     1280,
			Height:    800,
			MinWidth:  900,
			MinHeight: 560,
			URL:       "/",
		})
	}

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

// openStore opens the local database in its default place. The path is
// returned for error messages, even when opening fails.
func openStore() (*store.Store, string, error) {
	path, err := store.DefaultPath()
	if err != nil {
		return nil, "", err
	}
	st, err := store.Open(context.Background(), path)
	return st, path, err
}

func storeErrorMessage(path string, err error) string {
	if errors.Is(err, store.ErrNewerSchema) {
		return fmt.Sprintf("Your connections were last saved by a newer version of Querra. Install the latest version to open them.\n\n%s", path)
	}
	return fmt.Sprintf("Querra could not open its database at %s.\n\n%v", path, err)
}
