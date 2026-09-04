package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const cliEventPrefix = "AMDL_EVENT "

type DownloadManager struct {
	app    *App
	mu     sync.RWMutex
	state  DownloadState
	cancel context.CancelFunc
	job    *processJob
}

type cliEvent struct {
	Version int             `json:"version"`
	Type    string          `json:"type"`
	JobID   string          `json:"jobId"`
	Payload json.RawMessage `json:"payload"`
}

func NewDownloadManager(app *App) *DownloadManager {
	return &DownloadManager{app: app, state: DownloadState{Phase: "idle", Results: []TrackResult{}}}
}

func (m *DownloadManager) State() DownloadState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneDownloadState(m.state)
}

func (m *DownloadManager) Start(request DownloadRequest, settings Settings) error {
	urls, err := validateDownloadRequest(request)
	if err != nil {
		return err
	}
	if status := inspectProject(settings.ProjectRoot); !status.Valid {
		return fmt.Errorf("%s", status.Description)
	}
	if !m.app.wrapper.Status().Ready {
		return fmt.Errorf("Wrapper 尚未就绪，请先完成登录并确认四个端口")
	}
	validated, err := validateSettings(settings)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if m.state.Running {
		m.mu.Unlock()
		return fmt.Errorf("已有下载任务正在运行")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.state = DownloadState{
		Running: true, Phase: "queued", QueueSize: len(urls), Results: []TrackResult{},
		Message: "任务已加入队列",
	}
	m.mu.Unlock()
	m.publish()
	go m.runQueue(ctx, urls, strings.ToLower(request.Quality), validated)
	return nil
}

func (m *DownloadManager) Cancel() {
	m.mu.Lock()
	if !m.state.Running {
		m.mu.Unlock()
		return
	}
	m.state.Canceled = true
	m.state.Message = "正在取消…"
	cancel := m.cancel
	job := m.job
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if job != nil {
		job.Close()
	}
	m.publish()
}

func (m *DownloadManager) runQueue(ctx context.Context, urls []string, quality string, settings Settings) {
	overridesPath, err := writeSafeOverrides(m.app.settingsDir, settings)
	if err != nil {
		m.finishWithError("无法生成安全设置：" + err.Error())
		return
	}
	for index, rawURL := range urls {
		if ctx.Err() != nil {
			break
		}
		m.update(func(state *DownloadState) {
			state.Queue = index + 1
			state.Phase = "starting"
			state.Message = fmt.Sprintf("正在处理队列 %d/%d", index+1, len(urls))
		})
		beforeErrors := m.State().Errors
		if err := m.runOne(ctx, rawURL, quality, settings, overridesPath); err != nil {
			if ctx.Err() != nil {
				break
			}
			m.update(func(state *DownloadState) {
				if state.Errors == beforeErrors {
					state.Errors++
				}
				state.Message = err.Error()
			})
			m.log("error", err.Error())
		}
	}
	m.mu.Lock()
	m.state.Running = false
	m.state.Phase = "finished"
	if m.state.Canceled {
		m.state.Message = "任务已取消"
	} else if m.state.Errors > 0 {
		m.state.Message = "队列完成，但存在失败项目"
	} else {
		m.state.Message = "队列已完成"
	}
	m.cancel = nil
	m.job = nil
	m.mu.Unlock()
	m.publish()
}

func (m *DownloadManager) runOne(ctx context.Context, rawURL, quality string, settings Settings, overridesPath string) error {
	executable := filepath.Join(settings.ProjectRoot, "apple-music-downloader.exe")
	args := []string{
		"--config", filepath.Join(settings.ProjectRoot, "config.yaml"),
		"--safe-overrides", overridesPath,
		"--machine-events", "--non-interactive", "--json",
	}
	switch quality {
	case "alac":
	case "atmos":
		args = append(args, "--atmos")
	case "aac":
		args = append(args, "--aac", "--aac-type", settings.AacType)
	default:
		return fmt.Errorf("不支持的音质：%s", quality)
	}
	if isSingleSongURL(rawURL) {
		args = append(args, "--song")
	}
	args = append(args, rawURL)

	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = settings.ProjectRoot
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动下载器失败: %w", err)
	}
	job, jobErr := createProcessJob(cmd.Process.Pid)
	if jobErr != nil {
		_ = cmd.Process.Kill()
		return jobErr
	}
	m.mu.Lock()
	m.job = job
	m.mu.Unlock()

	var readers sync.WaitGroup
	readers.Add(2)
	go func() { defer readers.Done(); m.scanOutput(stdout) }()
	go func() { defer readers.Done(); m.scanOutput(stderr) }()
	err = cmd.Wait()
	readers.Wait()
	job.Close()
	m.mu.Lock()
	if m.job == job {
		m.job = nil
	}
	m.mu.Unlock()
	if ctx.Err() != nil {
		return context.Canceled
	}
	if err != nil {
		return fmt.Errorf("下载器返回失败: %w", err)
	}
	return nil
}

func (m *DownloadManager) scanOutput(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, cliEventPrefix) {
			m.handleCLIEvent(strings.TrimPrefix(line, cliEventPrefix))
			continue
		}
		if safe := redactLine(line); safe != "" {
			m.log("info", safe)
		}
	}
	if err := scanner.Err(); err != nil {
		m.log("warning", "读取下载器输出失败："+err.Error())
	}
}

func (m *DownloadManager) handleCLIEvent(raw string) {
	var event cliEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil || event.Version != 1 {
		m.log("warning", "收到无法识别的下载器事件")
		return
	}
	switch event.Type {
	case "queue_item_started":
		// The GUI starts one CLI process per URL, so its own queue counters are
		// authoritative across the complete desktop queue.
	case "track_started":
		var value struct {
			Index  int    `json:"index"`
			Total  int    `json:"total"`
			Song   string `json:"song"`
			Artist string `json:"artist"`
		}
		_ = json.Unmarshal(event.Payload, &value)
		m.update(func(state *DownloadState) {
			state.Track, state.Tracks = value.Index, value.Total
			state.Message = strings.TrimSpace(value.Artist + " — " + value.Song)
		})
	case "phase":
		var value struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(event.Payload, &value)
		m.update(func(state *DownloadState) { state.Phase = value.Name })
	case "track_completed":
		var value TrackResult
		_ = json.Unmarshal(event.Payload, &value)
		value.Status = "completed"
		m.update(func(state *DownloadState) {
			state.Completed++
			state.Results = append(state.Results, value)
		})
	case "warning":
		m.update(func(state *DownloadState) { state.Warnings++ })
	case "error":
		m.update(func(state *DownloadState) { state.Errors++ })
	case "job_finished":
		var value struct {
			Completed int `json:"completed"`
			Warnings  int `json:"warnings"`
			Errors    int `json:"errors"`
		}
		_ = json.Unmarshal(event.Payload, &value)
		m.update(func(state *DownloadState) {
			if value.Completed > state.Completed {
				state.Completed = value.Completed
			}
			state.Warnings = max(state.Warnings, value.Warnings)
			state.Errors = max(state.Errors, value.Errors)
		})
	}
}

func (m *DownloadManager) finishWithError(message string) {
	m.mu.Lock()
	m.state.Running = false
	m.state.Errors++
	m.state.Phase = "finished"
	m.state.Message = message
	m.cancel = nil
	m.mu.Unlock()
	m.log("error", message)
	m.publish()
}

func (m *DownloadManager) update(update func(*DownloadState)) {
	m.mu.Lock()
	update(&m.state)
	m.mu.Unlock()
	m.publish()
}

func (m *DownloadManager) publish() {
	m.app.emit("download:state", m.State())
}

func (m *DownloadManager) log(level, message string) {
	m.app.emit("app:log", map[string]string{"level": level, "message": redactLine(message)})
}

func cloneDownloadState(value DownloadState) DownloadState {
	copyValue := value
	copyValue.Results = append([]TrackResult(nil), value.Results...)
	return copyValue
}

func validateDownloadRequest(request DownloadRequest) ([]string, error) {
	if !containsString([]string{"alac", "atmos", "aac"}, strings.ToLower(request.Quality)) {
		return nil, fmt.Errorf("请选择有效音质")
	}
	seen := map[string]bool{}
	urls := make([]string, 0, len(request.URLs))
	for _, candidate := range request.URLs {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || seen[candidate] {
			continue
		}
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "music.apple.com") {
			return nil, fmt.Errorf("无效的 Apple Music 链接：%s", candidate)
		}
		path := strings.ToLower(parsed.Path)
		if !strings.Contains(path, "/album/") && !strings.Contains(path, "/playlist/") && !strings.Contains(path, "/song/") {
			return nil, fmt.Errorf("首版仅支持歌曲、专辑和播放列表链接：%s", candidate)
		}
		seen[candidate] = true
		urls = append(urls, candidate)
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("请至少输入一个 Apple Music 链接")
	}
	return urls, nil
}

func isSingleSongURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(parsed.Path), "/song/") || parsed.Query().Get("i") != ""
}
