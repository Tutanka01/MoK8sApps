// kaboom is a tiny chaos dashboard: burn CPU, hog memory, fail readiness
// or crash the process, to watch how the cluster reacts.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

// Overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

const (
	burnDuration  = 30 * time.Second
	paleDuration  = 30 * time.Second
	chunkSize     = 64 << 20
	explodeDelay  = 200 * time.Millisecond
	listenAddress = ":8080"
)

//go:embed static
var static embed.FS

type app struct {
	pod       string
	namespace string
	node      string
	started   time.Time

	burners   atomic.Int32
	paleUntil atomic.Int64 // unix nano; readiness fails until then

	mu  sync.Mutex
	hog [][]byte
}

type state struct {
	Pod           string `json:"pod"`
	Namespace     string `json:"namespace"`
	Node          string `json:"node"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptimeSeconds"`
	Ready         bool   `json:"ready"`
	Burners       int32  `json:"burners"`
	HogMiB        int    `json:"hogMiB"`
}

type event struct {
	Pod     string `json:"pod"`
	Message string `json:"message"`
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (a *app) ready() bool {
	return time.Now().UnixNano() >= a.paleUntil.Load()
}

func (a *app) state() state {
	a.mu.Lock()
	hog := len(a.hog) * chunkSize >> 20
	a.mu.Unlock()
	return state{
		Pod:           a.pod,
		Namespace:     a.namespace,
		Node:          a.node,
		Version:       version,
		UptimeSeconds: int64(time.Since(a.started).Seconds()),
		Ready:         a.ready(),
		Burners:       a.burners.Load(),
		HogMiB:        hog,
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func (a *app) reply(w http.ResponseWriter, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	writeJSON(w, http.StatusOK, event{Pod: a.pod, Message: msg})
}

func (a *app) handleBurn(w http.ResponseWriter, _ *http.Request) {
	a.burners.Add(1)
	go func() {
		defer a.burners.Add(-1)
		deadline := time.Now().Add(burnDuration)
		for time.Now().Before(deadline) {
			// Spin: keep one core busy.
		}
	}()
	a.reply(w, "chauffe 1 cœur pendant %s", burnDuration)
}

func (a *app) handleEat(w http.ResponseWriter, _ *http.Request) {
	chunk := make([]byte, chunkSize)
	// Touch every page so the memory is actually resident.
	for i := 0; i < len(chunk); i += os.Getpagesize() {
		chunk[i] = 1
	}
	a.mu.Lock()
	a.hog = append(a.hog, chunk)
	total := len(a.hog) * chunkSize >> 20
	a.mu.Unlock()
	a.reply(w, "avale 64 Mo, %d Mo au total", total)
}

func (a *app) handleFree(w http.ResponseWriter, _ *http.Request) {
	a.mu.Lock()
	a.hog = nil
	a.mu.Unlock()
	runtime.GC()
	debug.FreeOSMemory()
	a.reply(w, "rend toute la RAM avalée")
}

func (a *app) handlePale(w http.ResponseWriter, _ *http.Request) {
	a.paleUntil.Store(time.Now().Add(paleDuration).UnixNano())
	a.reply(w, "se fait porter pâle pendant %s : /readyz répond 503", paleDuration)
}

func (a *app) handleExplode(w http.ResponseWriter, _ *http.Request) {
	a.reply(w, "explose dans %s", explodeDelay)
	go func() {
		time.Sleep(explodeDelay)
		log.Print("KABOOM")
		os.Exit(1)
	}()
}

func main() {
	pod, err := os.Hostname()
	if err != nil {
		pod = "inconnu"
	}
	a := &app{
		pod:       env("POD_NAME", pod),
		namespace: env("POD_NAMESPACE", "?"),
		node:      env("NODE_NAME", "?"),
		started:   time.Now(),
	}

	site, err := fs.Sub(static, "static")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(site))
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, a.state())
	})
	mux.HandleFunc("POST /api/burn", a.handleBurn)
	mux.HandleFunc("POST /api/eat", a.handleEat)
	mux.HandleFunc("POST /api/free", a.handleFree)
	mux.HandleFunc("POST /api/pale", a.handlePale)
	mux.HandleFunc("POST /api/explode", a.handleExplode)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !a.ready() {
			http.Error(w, "pale", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok\n"))
	})

	log.Printf("kaboom %s listening on %s (pod %s)", version, listenAddress, a.pod)
	log.Fatal(http.ListenAndServe(listenAddress, mux))
}
