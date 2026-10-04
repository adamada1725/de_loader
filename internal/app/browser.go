package app

import (
	"os/exec"
	"runtime"
)

func OpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
func OpenFolder(folder string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", folder)
	case "darwin":
		cmd = exec.Command("open", folder)
	default:
		cmd = exec.Command("xdg-open", folder)
	}
	_ = cmd.Start()
}
