package components

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/adamada1725/de_loader/internal/logging"
)

type Component struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}
type Manager struct {
	dir, os, arch string
	mu            sync.RWMutex
	state         map[string]Component
	client        *http.Client
	log           *logging.Logger
}

func New(dataDir, goos, arch string, logger *logging.Logger) *Manager {
	return &Manager{dir: filepath.Join(dataDir, "bin"), os: goos, arch: arch, state: map[string]Component{}, client: &http.Client{Timeout: 90 * time.Second}, log: logger}
}
func (m *Manager) Ensure(ctx context.Context) error {
	m.log.Event("checking components dir=%s", m.dir)
	if e := os.MkdirAll(m.dir, 0700); e != nil {
		return e
	}
	var first error
	for _, n := range []string{"yt-dlp", "ffmpeg"} {
		if e := m.ensure(ctx, n); e != nil {
			m.log.Error("component=%s installation failed: %v", n, e)
			m.mu.Lock()
			c := m.state[n]
			c.Status, c.Error = "failed", e.Error()
			m.state[n] = c
			m.mu.Unlock()
			if first == nil {
				first = e
			}
		}
	}
	return first
}
func (m *Manager) ensure(ctx context.Context, name string) error {
	p := m.path(name)
	c := Component{Name: name, Path: p, Status: "missing"}
	if usable(p) {
		m.log.Event("component=%s found path=%s", name, p)
		c.Status = "ready"
		c.Version = version(ctx, p)
		m.set(c)
		return nil
	}
	m.set(c)
	u, err := downloadURL(name, m.os, m.arch)
	if err != nil {
		return err
	}
	m.log.Event("component=%s downloading url=%s", name, u)
	if err = m.download(ctx, u, p); err != nil {
		return err
	}
	if !usable(p) {
		return fmt.Errorf("%s validation failed", name)
	}
	c.Status = "ready"
	c.Version = version(ctx, p)
	m.set(c)
	m.log.Event("component=%s installed version=%s", name, c.Version)
	return nil
}
func (m *Manager) path(n string) string {
	if m.os == "windows" {
		n += ".exe"
	}
	return filepath.Join(m.dir, n)
}
func (m *Manager) download(ctx context.Context, url, target string) error {
	tmp := target + ".download"
	_ = os.Remove(tmp)
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if e != nil {
		return e
	}
	r, e := m.client.Do(req)
	if e != nil {
		return fmt.Errorf("download component: %w", e)
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("download component: server returned %s", r.Status)
	}
	m.log.Event("component download response url=%s status=%s size=%d", url, r.Status, r.ContentLength)
	f, e := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0700)
	if e != nil {
		return e
	}
	bytes, copyErr := io.Copy(f, r.Body)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if e = os.Chmod(tmp, 0700); e != nil {
		return e
	}
	if err := os.Rename(tmp, target); err != nil {
		return err
	}
	m.log.Event("component downloaded target=%s bytes=%d", target, bytes)
	return nil
}
func usable(path string) bool { s, e := os.Stat(path); return e == nil && !s.IsDir() && s.Size() > 0 }
func version(ctx context.Context, path string) string {
	out, e := exec.CommandContext(ctx, path, "--version").Output()
	if e != nil {
		return "unknown"
	}
	return strings.TrimSpace(strings.Split(string(out), "\n")[0])
}
func (m *Manager) set(c Component) { m.mu.Lock(); m.state[c.Name] = c; m.mu.Unlock() }
func (m *Manager) Components() []Component {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return []Component{m.state["yt-dlp"], m.state["ffmpeg"]}
}
func (m *Manager) Path(n string) (string, error) {
	p := m.path(n)
	if !usable(p) {
		return "", errors.New("component missing")
	}
	return p, nil
}
func downloadURL(name, goos, arch string) (string, error) {
	if arch != "amd64" && arch != "arm64" {
		return "", errors.New("unsupported architecture")
	}
	if name == "yt-dlp" {
		if goos == "windows" {
			return "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe", nil
		}
		return "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp", nil
	}
	if name == "ffmpeg" {
		platform := map[string]string{"windows": "win32-x64", "linux": "linux-x64", "darwin": "darwin-x64"}[goos]
		if goos == "darwin" && arch == "arm64" {
			platform = "darwin-arm64"
		}
		if platform == "" {
			return "", fmt.Errorf("automatic FFmpeg installation is not available for %s/%s", goos, arch)
		}
		return "https://github.com/eugeneware/ffmpeg-static/releases/latest/download/ffmpeg-" + platform, nil
	}
	return "", errors.New("unknown component")
}

var _ = runtime.GOOS
