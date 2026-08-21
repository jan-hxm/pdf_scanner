package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:     "PDF Scanner",
		Width:     1200,
		Height:    800,
		MinWidth:  680,
		MinHeight: 580,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind:      []interface{}{app},
		DragAndDrop: &options.DragAndDrop{
			// WebView2 exposes no path on dropped File objects; Wails hands the
			// real absolute paths to runtime.OnFileDrop instead.
			EnableFileDrop: true,
			// Without this, a file dropped outside the drop area is opened by
			// the webview itself, replacing the app with a PDF view.
			DisableWebViewDrop: true,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
