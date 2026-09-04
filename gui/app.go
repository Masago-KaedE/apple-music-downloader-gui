package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx         context.Context
	mu          sync.RWMutex
	settings    Settings
	settingsDir string
	downloader  *DownloadManager
	wrapper     *WrapperManager
}

func NewApp() *App {
	settingsDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "AppleMusicDownloaderGUI")
	app := &App{settingsDir: settingsDir}
	app.downloader = NewDownloadManager(app)
	app.wrapper = NewWrapperManager(app)
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	settings, err := loadSettings(a.settingsDir)
	if err != nil {
		settings = defaultSettings()
		a.emit("app:log", map[string]any{"level": "warning", "message": "设置加载失败，已使用默认值：" + err.Error()})
	}
	a.mu.Lock()
	a.settings = settings
	a.mu.Unlock()
	go a.refreshStatus()
}

func (a *App) shutdown(context.Context) {
	a.downloader.Cancel()
	a.wrapper.StopOwned()
}

func (a *App) emit(name string, data interface{}) {
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, name, data)
	}
}

func (a *App) refreshStatus() {
	a.emit("app:snapshot", a.GetSnapshot())
}

func (a *App) GetSnapshot() AppSnapshot {
	settings := a.getSettings()
	return AppSnapshot{
		Settings: settings,
		Project:  inspectProject(settings.ProjectRoot),
		Wrapper:  a.wrapper.Status(),
		Download: a.downloader.State(),
		Distros:  listDistributions(),
	}
}

func (a *App) SaveSettings(settings Settings) (Settings, error) {
	validated, err := validateSettings(settings)
	if err != nil {
		return Settings{}, err
	}
	if status := inspectProject(validated.ProjectRoot); !status.Valid {
		return Settings{}, fmt.Errorf("%s", status.Description)
	}
	if err := persistSettings(a.settingsDir, validated); err != nil {
		return Settings{}, err
	}
	a.mu.Lock()
	a.settings = validated
	a.mu.Unlock()
	a.refreshStatus()
	return validated, nil
}

func (a *App) BrowseProject() (string, error) {
	selection, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title:            "选择 Apple Music Downloader 项目目录",
		DefaultDirectory: a.getSettings().ProjectRoot,
	})
	if err != nil || selection == "" {
		return selection, err
	}
	if status := inspectProject(selection); !status.Valid {
		return "", fmt.Errorf("%s", status.Description)
	}
	return selection, nil
}

func (a *App) StartDownloads(request DownloadRequest) error {
	return a.downloader.Start(request, a.getSettings())
}

func (a *App) CancelDownloads() {
	a.downloader.Cancel()
}

func (a *App) RefreshWrapper() WrapperStatus {
	status := a.wrapper.Status()
	a.emit("wrapper:status", status)
	return status
}

func (a *App) StartWrapper(distribution, appleID, password string) error {
	return a.wrapper.Start(distribution, appleID, password)
}

func (a *App) SubmitTwoFactor(code string) error {
	return a.wrapper.SubmitTwoFactor(code)
}

func (a *App) StopWrapper() error {
	return a.wrapper.StopOwned()
}

func (a *App) OpenWrapperTerminal(distribution string) error {
	settings := a.getSettings()
	if distribution == "" {
		distribution = settings.Distribution
	}
	linuxPath := windowsPathToWSL(filepath.Join(settings.ProjectRoot, "wrapper-runtime"))
	terminal, err := exec.LookPath("wt.exe")
	if err != nil {
		return fmt.Errorf("找不到 Windows Terminal；请手动打开 Ubuntu 并进入 %s", linuxPath)
	}
	cmd := exec.Command(terminal, "-w", "new", "wsl.exe", "-d", distribution, "--cd", linuxPath)
	return cmd.Start()
}

func (a *App) OpenOutputFolder(quality string) error {
	settings := a.getSettings()
	folder := settings.AlacSaveFolder
	if quality == "atmos" {
		folder = settings.AtmosSaveFolder
	} else if quality == "aac" {
		folder = settings.AacSaveFolder
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		return fmt.Errorf("仅支持 Windows")
	}
	return exec.Command("explorer.exe", folder).Start()
}

func (a *App) getSettings() Settings {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.settings
}
