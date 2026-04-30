//go:build desktop
// +build desktop

package desktop

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed frontend/dist
var assets embed.FS

func runDesktop() {
	app := NewApp()

	if err := wails.Run(&options.App{
		Title:  "SmartClaw",
		Width:  1200,
		Height: 800,
		MinWidth: 800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 255},
		OnStartup:  app.Startup,
		OnShutdown: app.Shutdown,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	}); err != nil {
		log.Println("desktop: " + err.Error())
		os.Exit(1)
	}
}

// Main is the entry point for the desktop application.
// It is called from a separate main package when building with the desktop tag.
func Main() {
	runDesktop()
}
