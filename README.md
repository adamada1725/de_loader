# de_loader

Local, browser-based audio downloader written in Go. The executable serves its embedded UI only on `127.0.0.1` and installs `yt-dlp` and `ffmpeg` into its own application-data `bin` directory on first launch.

Build a development version:

```sh
go build ./cmd/de_loader
```

Release builds inject a semantic version with `-ldflags "-X main.version=v0.1.0"`. Tagged releases are built by GitHub Actions, and the application checks the repository's latest stable GitHub Release at most once per 24 hours.
