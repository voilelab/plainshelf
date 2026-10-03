//go:build js && wasm

// Command plainshelf-wasm runs the server inside a browser. There is no
// listener: requests arrive through a JS function wrapping App.Handler.
package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"syscall/js"

	"github.com/voilelab/plainshelf/cmd/plainshelf-wasm/shelfzip"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

func main() {
	root := os.Getenv("PLAINSHELF_ROOT")
	if root == "" {
		root = "/plainshelf"
	}
	libRoot := root + "/shelf"
	stderrLog := logutil.LogConf{Level: "info", Format: "text", LogFile: logutil.LogFileConf{Type: logutil.LogFileTypeStderr}}
	app, err := server.NewApp(&server.AppConf{
		Logger: stderrLog,
		Shelves: []*shelf.ShelfConfWithID{{
			ID:   "demo",
			Name: "Demo Shelf",
			ShelfConf: shelf.ShelfConf{
				LibRoot:  libRoot,
				LockMode: "none",
				Logger:   stderrLog,
			},
		}},
		StorePath:  root + "/store",
		CoverToJPG: true,
		Security:   &server.SecurityConf{Mode: server.SecurityModeNone},
	})
	if err != nil {
		panic(err)
	}
	if err := app.Start(); err != nil {
		panic(err)
	}
	handler := demoRoutes(libRoot, app.Handler())

	// plainshelfFetch(method, url, headers, body?: Uint8Array) -> Promise<{status, headers, body: Uint8Array}>
	js.Global().Set("plainshelfFetch", js.FuncOf(func(_ js.Value, args []js.Value) any {
		method, url, hdrs := args[0].String(), args[1].String(), args[2]
		var body []byte
		if len(args) > 3 && !args[3].IsUndefined() && !args[3].IsNull() {
			body = make([]byte, args[3].Get("length").Int())
			js.CopyBytesToGo(body, args[3])
		}
		return newPromise(func() (any, error) {
			req := httptest.NewRequest(method, url, bytes.NewReader(body))
			keys := js.Global().Get("Object").Call("keys", hdrs)
			for i := range keys.Length() {
				k := keys.Index(i).String()
				req.Header.Set(k, hdrs.Get(k).String())
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			res := rec.Result()
			data, _ := io.ReadAll(res.Body)
			outHdrs := js.Global().Get("Object").New()
			for k := range res.Header {
				outHdrs.Set(k, res.Header.Get(k))
			}
			arr := js.Global().Get("Uint8Array").New(len(data))
			js.CopyBytesToJS(arr, data)
			out := js.Global().Get("Object").New()
			out.Set("status", res.StatusCode)
			out.Set("headers", outHdrs)
			out.Set("body", arr)
			return out, nil
		})
	}))
	js.Global().Call("plainshelfReady")
	select {}
}

// newPromise runs fn off the JS event loop; blocking a js.FuncOf callback deadlocks.
func newPromise(fn func() (any, error)) js.Value {
	executor := js.FuncOf(func(_ js.Value, args []js.Value) any {
		resolve, reject := args[0], args[1]
		go func() {
			v, err := fn()
			if err != nil {
				reject.Invoke(js.Global().Get("Error").New(err.Error()))
				return
			}
			resolve.Invoke(v)
		}()
		return nil
	})
	// The Promise constructor calls the executor synchronously, so it can go now.
	defer executor.Release()
	return js.Global().Get("Promise").New(executor)
}

// demoRoutes adds the demo's shelf zip export and import in front of the app.
// They sit outside /api: they are not part of the server's API contract.
func demoRoutes(libRoot string, app http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /_demo/shelf.zip", func(w http.ResponseWriter, _ *http.Request) {
		var buf bytes.Buffer
		if err := shelfzip.Export(&buf, libRoot); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(buf.Bytes())
	})
	mux.HandleFunc("PUT /_demo/shelf.zip", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// The page reloads after an import, so the server restarts on the new shelf
		// rather than reconciling the one it has in memory.
		err = shelfzip.Import(bytes.NewReader(body), int64(len(body)), libRoot)
		switch {
		case errors.Is(err, shelfzip.ErrNotShelf), errors.Is(err, shelfzip.ErrUnsafePath),
			errors.Is(err, shelfzip.ErrTooLarge), errors.Is(err, zip.ErrFormat):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.Handle("/", app)
	return mux
}
