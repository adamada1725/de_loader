package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/adamada1725/de_loader/internal/components"
	"github.com/adamada1725/de_loader/internal/logging"
	"github.com/adamada1725/de_loader/internal/netproxy"
)

type Info struct {
	Title      string `json:"title"`
	Uploader   string `json:"uploader,omitempty"`
	Thumbnail  string `json:"thumbnail,omitempty"`
	Duration   int    `json:"duration,omitempty"`
	IsPlaylist bool   `json:"isPlaylist"`
	EntryCount int    `json:"entryCount,omitempty"`
}

func Fetch(ctx context.Context, cm *components.Manager, logger *logging.Logger, url string) (*Info, error) {
	path, err := cm.Path("yt-dlp")
	if err != nil {
		return nil, fmt.Errorf("download component is unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	proxy, err := netproxy.Start(logger)
	if err != nil {
		return nil, fmt.Errorf("could not prepare network connection")
	}
	defer proxy.Close()
	out, err := exec.CommandContext(ctx, path, "--proxy", proxy.URL(), "--no-playlist", "--no-warnings", "--skip-download", "--dump-single-json", url).CombinedOutput()
	if err != nil {
		logger.Error("metadata retrieval failed url=%s error=%v\noutput:\n%s", url, err, string(out))
		return nil, fmt.Errorf("could not read video information")
	}
	var raw struct {
		Title, Uploader, Thumbnail string
		Duration                   int
		Entries                    []json.RawMessage
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		logger.Error("metadata JSON parsing failed url=%s: %v\noutput:\n%s", url, err, string(out))
		return nil, fmt.Errorf("could not read video information")
	}
	return &Info{Title: raw.Title, Uploader: raw.Uploader, Thumbnail: raw.Thumbnail, Duration: raw.Duration, IsPlaylist: len(raw.Entries) > 0, EntryCount: len(raw.Entries)}, nil
}
