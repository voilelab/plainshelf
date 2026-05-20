package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/frontend"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	app := NewDesktopApp()

	err := wails.Run(&options.App{
		Title:  "PlainShelf",
		Width:  1600,
		Height: 1200,
		AssetServer: &assetserver.Options{
			Assets: frontend.WebFS,
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// If the request is for an API endpoint, use the app's API handler
					if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
						app.GetAPIHandler().ServeHTTP(w, r)
						return
					}
					// Otherwise, serve the static assets
					next.ServeHTTP(w, r)
				})
			},
		},
		OnStartup: app.Startup,
		OnShutdown: func(context.Context) {
			app.Shutdown()
		},

		Bind: []any{app},
	})

	if err != nil {
		log.Fatal(err)
	}
}
