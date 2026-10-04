package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/adamada1725/de_loader/internal/app"
)

// version is set for release builds with: -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

//go:embed all:web
var webFS embed.FS

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(app.Options{Version: version, OS: runtime.GOOS, Arch: runtime.GOARCH, WebFS: webFS})
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()
	go a.Startup(ctx)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Handler: a.Handler(), ReadHeaderTimeout: 5 * time.Second}
	url := fmt.Sprintf("http://%s", listener.Addr())
	log.Printf("de_loader %s listening on %s", version, url)
	go app.OpenBrowser(url)
	go func() { <-ctx.Done(); _ = server.Shutdown(context.Background()) }()
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
