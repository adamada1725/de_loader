package update

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/adamada1725/de_loader/internal/config"
	"net/http"
	"strings"
	"time"
)

type Result struct {
	Current   string    `json:"current"`
	Latest    string    `json:"latest"`
	URL       string    `json:"url"`
	Available bool      `json:"available"`
	CheckedAt time.Time `json:"checkedAt"`
	Error     string    `json:"error,omitempty"`
}
type Checker struct {
	cfg     *config.Store
	version string
	result  Result
	client  *http.Client
}

func New(c *config.Store, v string) *Checker {
	return &Checker{cfg: c, version: v, result: Result{Current: v}, client: &http.Client{Timeout: 10 * time.Second}}
}
func (c *Checker) CheckIfDue(ctx context.Context) {
	last, _ := time.Parse(time.RFC3339, c.cfg.Get().LastUpdateCheck)
	if time.Since(last) > 24*time.Hour {
		c.Check(ctx)
	}
}
func (c *Checker) Check(ctx context.Context) Result {
	r := Result{Current: c.version, CheckedAt: time.Now()}
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/adamada1725/de_loader/releases/latest", nil)
	resp, e := c.client.Do(req)
	if e != nil {
		r.Error = "Could not check for updates"
		c.result = r
		return r
	}
	defer resp.Body.Close()
	var data struct {
		TagName, HTMLURL string
		Prerelease       bool
	}
	if resp.StatusCode != 200 || json.NewDecoder(resp.Body).Decode(&data) != nil || data.Prerelease {
		r.Error = "Could not check for updates"
	} else {
		r.Latest = data.TagName
		r.URL = data.HTMLURL
		r.Available = greater(data.TagName, c.version)
	}
	c.result = r
	v := c.cfg.Get()
	v.LastUpdateCheck = time.Now().Format(time.RFC3339)
	_ = c.cfg.Set(v)
	return r
}
func (c *Checker) Result() Result { return c.result }
func greater(a, b string) bool {
	pa := parts(a)
	pb := parts(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}
func parts(v string) [3]int {
	var r [3]int
	v = strings.TrimPrefix(v, "v")
	_, _ = fmtSscanf(v, "%d.%d.%d", &r[0], &r[1], &r[2])
	return r
}
func fmtSscanf(s, f string, a ...any) (int, error) { return fmt.Sscanf(s, f, a...) }
