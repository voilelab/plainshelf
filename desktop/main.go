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
					if isAPIPath(r.URL.Path) {
						app.GetAPIHandler().ServeHTTP(w, r)
						return
					}
					// Otherwise, serve the static assets
					next.ServeHTTP(w, r)
				})
			},
		},
		Menu:      newApplicationMenu(app),
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

func isAPIPath(path string) bool {
	normalized := strings.TrimSpace(path)
	switch {
	case normalized == "/api" || strings.HasPrefix(normalized, "/api/"):
		return true
	case normalized == "api" || strings.HasPrefix(normalized, "api/"):
		return true
	case normalized == "/wails/api" || strings.HasPrefix(normalized, "/wails/api/"):
		return true
	default:
		return false
	}
}
