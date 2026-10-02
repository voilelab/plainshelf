//go:build js && wasm

// Command plainshelf-wasm runs the server inside a browser. There is no
// listener: requests arrive through a JS function wrapping App.Handler.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall/js"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

func main() {
	root := os.Getenv("PLAINSHELF_ROOT")
	if root == "" {
		root = "/plainshelf"
	}
	writerID, err := loadWriterID(root + "/book-cache-writer-id")
	if err != nil {
		panic(err)
	}
	stderrLog := logutil.LogConf{Level: "info", Format: "text", LogFile: logutil.LogFileConf{Type: logutil.LogFileTypeStderr}}
	app, err := server.NewApp(&server.AppConf{
		Logger: stderrLog,
		Shelves: []*shelf.ShelfConfWithID{{
			ID:   "demo",
			Name: "Demo Shelf",
			ShelfConf: shelf.ShelfConf{
				LibRoot:  root + "/shelf",
				LockMode: "none",
				Logger:   stderrLog,
				// The store is in memory, so the ID it would generate changes on
				// every load; a new ID would leave another book cache each time.
				BookCacheWriterID: writerID,
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
	handler := app.Handler()

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

// loadWriterID reads the persisted book cache writer ID, creating it on first run.
func loadWriterID(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		return string(b), nil
	}
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := hex.EncodeToString(buf)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return id, os.WriteFile(path, []byte(id), 0o644)
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
