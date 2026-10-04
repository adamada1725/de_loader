package app

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/adamada1725/de_loader/internal/media"
)

func routes(a *App) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) { jsonOut(w, map[string]string{"status": "ok"}) })
	mux.HandleFunc("/api/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		contents, err := a.log.Read()
		if err != nil {
			http.Error(w, "diagnostics unavailable", http.StatusInternalServerError)
			return
		}
		jsonOut(w, map[string]string{"log": string(contents), "path": a.log.Path()})
	})
	mux.HandleFunc("/api/components", func(w http.ResponseWriter, r *http.Request) { jsonOut(w, a.components.Components()) })
	mux.HandleFunc("/api/media/info", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		var req struct {
			URL string `json:"url"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		info, err := media.Fetch(r.Context(), a.components, a.log, req.URL)
		if err != nil {
			a.log.Error("media info request failed: %v", err)
			jsonErrMessage(w, "MEDIA_INFO_FAILED", "Не удалось загрузить информацию о видео. Откройте журнал диагностики.", 400)
			return
		}
		jsonOut(w, info)
	})
	mux.HandleFunc("/api/components/install", func(w http.ResponseWriter, r *http.Request) {
		go a.components.Ensure(context.Background())
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("/api/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			jsonOut(w, a.updates.Check(r.Context()))
			return
		}
		jsonOut(w, a.updates.Result())
	})
	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			jsonOut(w, a.cfg.Get())
		case http.MethodPut:
			var c interface{}
			_ = c
			var next struct {
				DownloadDirectory string `json:"downloadDirectory"`
				DefaultFormat     string `json:"defaultFormat"`
			}
			if json.NewDecoder(r.Body).Decode(&next) != nil {
				http.Error(w, "invalid settings", 400)
				return
			}
			old := a.cfg.Get()
			if next.DownloadDirectory != "" {
				old.DownloadDirectory = next.DownloadDirectory
			}
			if next.DefaultFormat != "" {
				old.DefaultFormat = next.DefaultFormat
			}
			if e := a.cfg.Set(old); e != nil {
				http.Error(w, "could not save settings", 500)
				return
			}
			jsonOut(w, old)
		}
	})
	mux.HandleFunc("/api/settings/choose-folder", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		folder, err := chooseFolder()
		if err != nil {
			jsonErr(w, "FOLDER_DIALOG_FAILED", 400)
			return
		}
		stat, err := os.Stat(folder)
		if err != nil || !stat.IsDir() {
			jsonErr(w, "INVALID_FOLDER", 400)
			return
		}
		cfg := a.cfg.Get()
		cfg.DownloadDirectory = folder
		if err := a.cfg.Set(cfg); err != nil {
			http.Error(w, "could not save settings", 500)
			return
		}
		jsonOut(w, cfg)
	})
	mux.HandleFunc("/api/folder/open", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		var req struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		folder := a.cfg.Get().DownloadDirectory
		if req.Path != "" {
			folder = path.Dir(req.Path)
		}
		go OpenFolder(folder)
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("/api/downloads", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method", 405)
			return
		}
		var req struct {
			URL      string `json:"url"`
			Playlist bool   `json:"playlist"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		j, e := a.jobs.Create(req.URL, req.Playlist)
		if e != nil {
			jsonErr(w, e.Error(), 400)
			return
		}
		jsonOut(w, j)
	})
	mux.HandleFunc("/api/downloads/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/downloads/"), "/")
		j := a.jobs.Get(parts[0])
		if j == nil {
			http.NotFound(w, r)
			return
		}
		if len(parts) > 1 && parts[1] == "cancel" && r.Method == "POST" {
			a.jobs.Cancel(j.ID)
			w.WriteHeader(204)
			return
		}
		jsonOut(w, j)
	})
	sub, _ := fs.Sub(a.web, "web")
	mux.Handle("/", noCache(http.FileServer(http.FS(sub))))
	return hostGuard(mux)
}
func hostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Host
		if h != "" && !strings.HasPrefix(h, "127.0.0.1:") && !strings.HasPrefix(h, "localhost:") {
			http.Error(w, "invalid host", 400)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
func jsonOut(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func jsonErr(w http.ResponseWriter, msg string, status int) {
	jsonErrMessage(w, msg, "Could not start download. Check the link and try again.", status)
}
func jsonErrMessage(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}

var _ = path.Base
