package app

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func chooseFolder() (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("powershell", "-NoProfile", "-Command", "Add-Type -AssemblyName System.Windows.Forms; $d=New-Object System.Windows.Forms.FolderBrowserDialog; if($d.ShowDialog() -eq 'OK'){$d.SelectedPath}")
	case "darwin":
		cmd = exec.Command("osascript", "-e", "POSIX path of (choose folder with prompt \"Choose where to save audio\")")
	default:
		cmd = exec.Command("zenity", "--file-selection", "--directory", "--title=Choose where to save audio")
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("folder dialog was closed or is unavailable")
	}
	return strings.TrimSpace(string(out)), nil
}
