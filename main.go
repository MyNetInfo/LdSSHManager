package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"LdSSHManager/internal/config"
	"LdSSHManager/internal/webview"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// 清理上次异常退出残留的 WebView2 子进程: 它们会占用 WebView2 用户数据目录锁,
	// 导致本次启动的窗口无法初始化。必须在 wails.Run 之前执行, 这样本次的 WebView2 才能正常初始化。
	if killed, err := webview.CleanupOrphanWebView2(webviewIdentity(), config.WebView2Dir()); err != nil {
		fmt.Printf("cleanup webview2 orphans: %v\n", err)
	} else if killed > 0 {
		app.webviewCleaned = killed
		fmt.Printf("cleaned %d leftover WebView2 process(es) from previous run\n", killed)
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:            "LdSSHManager",
		Width:            1280,
		Height:           768,
		WindowStartState: options.Maximised,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		OnShutdown:       app.Shutdown,
		Windows: &windows.Options{
			// 固定 WebView2 用户数据目录, 便于启动自检识别并清理本应用的残留进程。
			WebviewUserDataPath: config.WebView2Dir(),
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// webviewIdentity 返回可执行文件名(含扩展名), 用作匹配残留 WebView2 进程的身份标识。
func webviewIdentity() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Base(exe)
	}
	return "LdSSHManager-x64.exe"
}
