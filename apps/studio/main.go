package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wmac "github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var studioAssets embed.FS

func main() {
	app, err := NewApp()
	if err != nil {
		log.Fatalf("create ASC Studio app: %v", err)
	}

	err = wails.Run(&options.App{
		Title:            "ASC Studio",
		Width:            1480,
		Height:           980,
		MinWidth:         1180,
		MinHeight:        760,
		BackgroundColour: options.NewRGBA(0, 0, 0, 0),
		AssetServer: &assetserver.Options{
			Assets: studioAssets,
		},
		OnStartup:       app.startup,
		OnShutdown:      app.shutdown,
		CSSDragProperty: "--wails-draggable",
		CSSDragValue:    "drag",
		Bind: []interface{}{
			app,
		},
		Mac: &wmac.Options{
			WindowIsTranslucent:  true,
			WebviewIsTransparent: true,
			Appearance:           wmac.DefaultAppearance,
			TitleBar: &wmac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				FullSizeContent:            true,
				UseToolbar:                 true,
				HideToolbarSeparator:       true,
			},
		},
	})
	if err != nil {
		log.Fatalf("run ASC Studio: %v", err)
	}
}
