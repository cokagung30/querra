package main

import (
	"embed"
	"log"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/cokagung30/querra/internal/app"
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

func main() {
	wailsApp := application.New(application.Options{
		Name:        "Querra",
		Description: "Lightweight database client",
		Services: []application.Service{
			application.NewService(app.NewAppService(version)),
			application.NewService(app.NewPerfService(processStart)),
		},
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

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Querra",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 560,
		URL:       "/",
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
