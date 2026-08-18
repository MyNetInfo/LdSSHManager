//go:build windows

package webview

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

)

// CleanupOrphanWebView2 kills lingering msedgewebview2.exe processes that belong to
// THIS application and were left behind when the app crashed or was force-killed.
// Such orphans keep the WebView2 user-data-dir locked, which prevents a fresh launch
// from initializing its window (the new process silently fails to bring up WebView2).
//
// matchAny are substrings used to recognize "our" WebView2 processes. Typically:
//   - the executable base name (e.g. "LdSSHManager-x64.exe"), which appears in the
//     default %APPDATA%\[BinaryName.exe] user-data-dir; and
//   - the configured WebView2 user-data-dir path (config.WebView2Dir()).
//
// Safety: if another real instance of the main executable is still running, we do NOT
// kill anything, to avoid tearing down the live instance's WebView2 processes.
func CleanupOrphanWebView2(exeBase string, matchAny ...string) (int, error) {
	needles := make([]string, 0, len(matchAny)+1)
	if exeBase != "" {
		needles = append(needles, strings.ToLower(exeBase))
	}
	for _, m := range matchAny {
		if m != "" {
			needles = append(needles, strings.ToLower(m))
		}
	}
	if len(needles) == 0 {
		return 0, nil
	}

	// List our main-exe count and every msedgewebview2.exe with its command line.
	// The command line carries --user-data-dir=<our path>, which is how we tell our
	// processes apart from other apps' WebView2 processes.
	script := fmt.Sprintf(
		`$ex='%s'; $w=@(Get-CimInstance Win32_Process -Filter "Name='msedgewebview2.exe'" | Select-Object ProcessId,CommandLine); $m=(Get-CimInstance Win32_Process -Filter "Name='$ex'").Count; [pscustomobject]@{main=$m; web=$w} | ConvertTo-Json -Compress`,
		escapePS(exeBase),
	)

	out, err := runHiddenPS(script)
	if err != nil {
		return 0, fmt.Errorf("enumerate webview2 processes: %w", err)
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return 0, nil
	}

	var res struct {
		Main int `json:"main"`
		Web  []struct {
			ProcessId   int    `json:"ProcessId"`
			CommandLine string `json:"CommandLine"`
		} `json:"web"`
	}
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return 0, fmt.Errorf("parse webview2 process list: %w", err)
	}

	// Another real instance is alive → never touch its WebView2.
	if res.Main > 1 {
		return 0, nil
	}

	killed := 0
	for _, p := range res.Web {
		if p.ProcessId <= 0 {
			continue
		}
		cl := strings.ToLower(p.CommandLine)
		hit := false
		for _, n := range needles {
			if n != "" && strings.Contains(cl, n) {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		if pp, perr := os.FindProcess(p.ProcessId); perr == nil {
			if kerr := pp.Kill(); kerr == nil {
				killed++
			}
		}
	}
	return killed, nil
}

// escapePS escapes a single quote for safe inclusion in a PowerShell string literal.
func escapePS(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// runHiddenPS runs a PowerShell snippet without flashing a console window.
// It uses CREATE_NO_WINDOW so the user never sees a black box on startup.
func runHiddenPS(script string) ([]byte, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	return cmd.Output()
}
