package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/adamada1725/de_loader/internal/components"
	"github.com/adamada1725/de_loader/internal/config"
	"github.com/adamada1725/de_loader/internal/logging"
	"github.com/adamada1725/de_loader/internal/netproxy"
)

type Status string

const (
	Queued      Status = "queued"
	Preparing   Status = "preparing"
	Downloading Status = "downloading"
	Processing  Status = "processing"
	Completed   Status = "completed"
	Failed      Status = "failed"
	Cancelled   Status = "cancelled"
)

type Job struct {
	ID         string    `json:"id"`
	URL        string    `json:"url"`
	Playlist   bool      `json:"playlist"`
	Title      string    `json:"title,omitempty"`
	Status     Status    `json:"status"`
	Progress   float64   `json:"progress"`
	Stage      string    `json:"stage,omitempty"`
	OutputPath string    `json:"outputPath,omitempty"`
	Error      string    `json:"error,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
type Manager struct {
	mu         sync.RWMutex
	all        map[string]*Job
	cancel     map[string]context.CancelFunc
	components *components.Manager
	cfg        *config.Store
	semaphore  chan struct{}
	closed     bool
	log        *logging.Logger
}

func New(c *components.Manager, cfg *config.Store, logger *logging.Logger) *Manager {
	return &Manager{all: map[string]*Job{}, cancel: map[string]context.CancelFunc{}, components: c, cfg: cfg, semaphore: make(chan struct{}, 2), log: logger}
}
func (m *Manager) Create(url string, playlist bool) (*Job, error) {
	if !youtubeURL(url) {
		return nil, fmt.Errorf("INVALID_URL")
	}
	j := &Job{ID: fmt.Sprintf("%d", time.Now().UnixNano()), URL: url, Playlist: playlist, Status: Queued, Stage: "Waiting to download", UpdatedAt: time.Now()}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, fmt.Errorf("application is closing")
	}
	m.all[j.ID] = j
	m.mu.Unlock()
	m.log.Event("job=%s created url=%s", j.ID, url)
	go m.run(j.ID)
	return copyJob(j), nil
}
func (m *Manager) run(id string) {
	m.log.Event("job=%s waiting for worker", id)
	m.semaphore <- struct{}{}
	defer func() { <-m.semaphore }()
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancel[id] = cancel
	m.mu.Unlock()
	defer func() { m.mu.Lock(); delete(m.cancel, id); m.mu.Unlock() }()
	path, e := m.components.Path("yt-dlp")
	if e != nil {
		m.log.Error("job=%s cannot start: yt-dlp missing: %v", id, e)
		m.fail(id, "DEPENDENCY_MISSING", "Download components are being installed. Try again in a moment.")
		return
	}
	out := filepath.Join(m.cfg.Get().DownloadDirectory, "%(title)s.%(ext)s")
	m.change(id, Preparing, "Preparing download", 0)
	proxy, err := netproxy.Start(m.log)
	if err != nil {
		m.log.Error("job=%s could not start local network fallback: %v", id, err)
		m.fail(id, "NETWORK_ERROR", "Could not prepare the network connection. Try again.")
		return
	}
	defer proxy.Close()
	m.log.Event("job=%s starting yt-dlp path=%s output=%s", id, path, out)
	playlistArg := "--no-playlist"
	if m.isPlaylist(id) {
		playlistArg = "--yes-playlist"
	}
	cmd := exec.CommandContext(ctx, path, "--proxy", proxy.URL(), playlistArg, "-x", "--audio-format", "mp3", "--newline", "--print", "after_move:filepath", "-o", out, m.jobURL(id))
	b, e := cmd.CombinedOutput()
	if ctx.Err() != nil {
		m.log.Event("job=%s cancelled", id)
		m.change(id, Cancelled, "Cancelled", 0)
		return
	}
	if e != nil {
		m.log.Error("job=%s yt-dlp failed: %v\noutput:\n%s", id, e, string(b))
		message := "Could not download this media. Check the link and try again."
		if strings.Contains(string(b), "Network is unreachable") || strings.Contains(string(b), "Name or service not known") || strings.Contains(string(b), "No remote IPv4 addresses") {
			message = "YouTube is unreachable from this network. Check DNS, your internet connection or VPN, then try again."
		}
		m.fail(id, "DOWNLOAD_FAILED", message)
		return
	}
	m.setOutput(id, lastPath(string(b)))
	m.log.Event("job=%s completed output=%s", id, m.Get(id).OutputPath)
	m.change(id, Completed, "Completed", 100)
}
func (m *Manager) jobURL(id string) string { m.mu.RLock(); defer m.mu.RUnlock(); return m.all[id].URL }
func (m *Manager) isPlaylist(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.all[id].Playlist
}
func (m *Manager) setOutput(id, output string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j := m.all[id]; j != nil {
		j.OutputPath = output
	}
}
func lastPath(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasSuffix(strings.ToLower(lines[i]), ".mp3") {
			return lines[i]
		}
	}
	return ""
}
func (m *Manager) change(id string, s Status, stage string, p float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j := m.all[id]; j != nil {
		j.Status = s
		j.Stage = stage
		j.Progress = p
		j.UpdatedAt = time.Now()
	}
}
func (m *Manager) fail(id, code, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j := m.all[id]; j != nil {
		j.Status = Failed
		j.Stage = "Download failed"
		j.Error = msg
		j.UpdatedAt = time.Now()
		_ = code
	}
}
func (m *Manager) Get(id string) *Job { m.mu.RLock(); defer m.mu.RUnlock(); return copyJob(m.all[id]) }
func copyJob(j *Job) *Job {
	if j == nil {
		return nil
	}
	c := *j
	return &c
}
func (m *Manager) Cancel(id string) bool {
	m.mu.RLock()
	c, ok := m.cancel[id]
	m.mu.RUnlock()
	if ok {
		m.log.Event("job=%s cancellation requested", id)
		c()
		return true
	}
	return false
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	for _, c := range m.cancel {
		c()
	}
	m.mu.Unlock()
}
func youtubeURL(s string) bool {
	ok, _ := regexp.MatchString(`^https?://(www\.)?(youtube\.com|youtu\.be)/`, s)
	return ok
}
func (m *Manager) JSON(id string) []byte { b, _ := json.Marshal(m.Get(id)); return b }

var _ = os.ErrNotExist
