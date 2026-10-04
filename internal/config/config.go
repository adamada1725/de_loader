package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type Config struct {
	DownloadDirectory   string `json:"downloadDirectory"`
	DefaultFormat       string `json:"defaultFormat"`
	LastUpdateCheck     string `json:"lastUpdateCheck"`
	LastDependencyCheck string `json:"lastDependencyCheck"`
}
type Store struct {
	mu            sync.RWMutex
	path, dataDir string
	value         Config
}

func Open() (*Store, error) {
	d, err := dataDir()
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(d, 0700); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(d, "config.json"), dataDir: d, value: Config{DefaultFormat: "mp3"}}
	if b, e := os.ReadFile(s.path); e == nil {
		_ = json.Unmarshal(b, &s.value)
	}
	if s.value.DownloadDirectory == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr == nil {
			s.value.DownloadDirectory = filepath.Join(home, "Downloads")
		} else {
			s.value.DownloadDirectory = filepath.Join(d, "downloads")
		}
	}
	_ = os.MkdirAll(s.value.DownloadDirectory, 0700)
	return s, s.Save()
}
func dataDir() (string, error) {
	if runtime.GOOS == "darwin" {
		h, e := os.UserHomeDir()
		return filepath.Join(h, "Library", "Application Support", "MediaDownloader"), e
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "MediaDownloader"), nil
	}
	d, e := os.UserConfigDir()
	return filepath.Join(d, "media-downloader"), e
}
func (s *Store) DataDir() string    { return s.dataDir }
func (s *Store) Get() Config        { s.mu.RLock(); defer s.mu.RUnlock(); return s.value }
func (s *Store) Set(c Config) error { s.mu.Lock(); s.value = c; s.mu.Unlock(); return s.Save() }
func (s *Store) Save() error {
	s.mu.RLock()
	b, e := json.MarshalIndent(s.value, "", "  ")
	s.mu.RUnlock()
	if e != nil {
		return e
	}
	return os.WriteFile(s.path, b, 0600)
}
