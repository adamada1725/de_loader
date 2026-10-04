// Package netproxy provides a localhost-only CONNECT proxy with an IPv4 DNS fallback.
package netproxy

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Logger interface {
	Event(string, ...any)
	Error(string, ...any)
}
type Proxy struct {
	server    *http.Server
	listener  net.Listener
	log       Logger
	closeOnce sync.Once
}

func Start(log Logger) (*Proxy, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	p := &Proxy{listener: listener, log: log}
	p.server = &http.Server{Handler: http.HandlerFunc(p.handle), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := p.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Error("local proxy failed: %v", err)
		}
	}()
	log.Event("local download proxy started address=%s", listener.Addr())
	return p, nil
}
func (p *Proxy) URL() string { return "http://" + p.listener.Addr().String() }
func (p *Proxy) Close() {
	p.closeOnce.Do(func() { _ = p.server.Close(); p.log.Event("local download proxy stopped") })
}

func (p *Proxy) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodConnect {
		http.Error(w, "CONNECT only", http.StatusMethodNotAllowed)
		return
	}
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		http.Error(w, "invalid target", http.StatusBadRequest)
		return
	}
	ip, err := resolveIPv4(r.Context(), host)
	if err != nil {
		p.log.Error("proxy DNS failed host=%s: %v", host, err)
		http.Error(w, "DNS lookup failed", http.StatusBadGateway)
		return
	}
	p.log.Event("proxy connect host=%s ip=%s", host, ip)
	target, err := net.DialTimeout("tcp", net.JoinHostPort(ip, port), 20*time.Second)
	if err != nil {
		p.log.Error("proxy connection failed host=%s ip=%s: %v", host, ip, err)
		http.Error(w, "connection failed", http.StatusBadGateway)
		return
	}
	h, ok := w.(http.Hijacker)
	if !ok {
		_ = target.Close()
		http.Error(w, "hijacking unavailable", 500)
		return
	}
	client, _, err := h.Hijack()
	if err != nil {
		_ = target.Close()
		return
	}
	_, _ = io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n")
	go func() { _, _ = io.Copy(target, client); _ = target.Close() }()
	_, _ = io.Copy(client, target)
	_ = client.Close()
}

func resolveIPv4(ctx context.Context, host string) (string, error) {
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip4", host)
	if err == nil && len(ips) > 0 {
		return ips[0].String(), nil
	}
	return dohIPv4(ctx, host)
}
func dohIPv4(ctx context.Context, host string) (string, error) {
	transport := &http.Transport{TLSClientConfig: &tls.Config{ServerName: "cloudflare-dns.com"}, DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 8 * time.Second}).DialContext(ctx, network, "1.1.1.1:443")
	}}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://cloudflare-dns.com/dns-query?name="+host+"&type=A", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/dns-json")
	response, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("public DNS unavailable: %w", err)
	}
	defer response.Body.Close()
	var body struct {
		Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if response.StatusCode != 200 || json.NewDecoder(response.Body).Decode(&body) != nil {
		return "", fmt.Errorf("public DNS returned no usable response")
	}
	for _, answer := range body.Answer {
		if answer.Type == 1 && net.ParseIP(answer.Data) != nil {
			return answer.Data, nil
		}
	}
	return "", fmt.Errorf("no IPv4 address for %s", strings.TrimSpace(host))
}
