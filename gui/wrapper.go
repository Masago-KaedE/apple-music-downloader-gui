package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var wrapperPorts = []int{10020, 20020, 30020, 40020}

type WrapperManager struct {
	app          *App
	mu           sync.RWMutex
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	distribution string
	linuxPID     int
	owned        bool
	needs2FA     bool
	stopping     bool
}

func NewWrapperManager(app *App) *WrapperManager {
	return &WrapperManager{app: app}
}

func (m *WrapperManager) Status() WrapperStatus {
	ports := checkWrapperPorts()
	ready := true
	for _, port := range ports {
		ready = ready && port.Listening
	}
	m.mu.RLock()
	status := WrapperStatus{
		Ready: ready, OwnedByGUI: m.owned, Running: ready || m.cmd != nil,
		Needs2FA: m.needs2FA, Distribution: m.distribution, Ports: ports,
	}
	m.mu.RUnlock()
	if ready {
		status.Message = "Wrapper 四个端口均已就绪"
	} else if status.Needs2FA {
		status.Message = "Wrapper 正在等待双重认证验证码"
	} else if status.Running {
		status.Message = "Wrapper 正在启动"
	} else {
		status.Message = "Wrapper 未启动或端口不完整"
	}
	return status
}

func (m *WrapperManager) Start(distribution, appleID, password string) error {
	if strings.TrimSpace(distribution) == "" {
		return fmt.Errorf("请选择 WSL 发行版")
	}
	if strings.TrimSpace(appleID) == "" || password == "" {
		return fmt.Errorf("Apple ID 和密码不能为空")
	}
	if strings.ContainsAny(appleID, "\r\n\x00") || strings.ContainsAny(password, "\r\n\x00") {
		return fmt.Errorf("凭据包含不支持的控制字符")
	}
	if m.Status().Ready {
		return fmt.Errorf("Wrapper 已经就绪，无需重复启动")
	}
	distros := listDistributions()
	if len(distros) > 0 && !containsString(distros, distribution) {
		return fmt.Errorf("WSL 发行版不存在：%s", distribution)
	}

	m.mu.Lock()
	if m.cmd != nil {
		m.mu.Unlock()
		return fmt.Errorf("Wrapper 正在启动或运行")
	}
	root := m.app.getSettings().ProjectRoot
	linuxDir := windowsPathToWSL(root + `\wrapper-runtime`)
	script := "cd " + bashQuote(linuxDir) + " || exit 20; " +
		"IFS= read -r APPLE_ID; IFS= read -r APPLE_PASS; " +
		"printf 'AMDL_WRAPPER_PID=%s\\n' \"$$\"; " +
		"exec ./wrapper -L \"$APPLE_ID:$APPLE_PASS\""
	cmd := exec.Command("wsl.exe", "-d", distribution, "--", "bash", "-lc", script)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		m.mu.Unlock()
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.mu.Unlock()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.mu.Unlock()
		return err
	}
	if err := cmd.Start(); err != nil {
		m.mu.Unlock()
		return fmt.Errorf("启动 WSL Wrapper 失败: %w", err)
	}
	m.cmd = cmd
	m.stdin = stdin
	m.distribution = distribution
	m.owned = true
	m.needs2FA = false
	m.stopping = false
	m.mu.Unlock()

	credentials := []byte(appleID + "\n" + password + "\n")
	_, writeErr := stdin.Write(credentials)
	for i := range credentials {
		credentials[i] = 0
	}
	appleID, password = "", ""
	if writeErr != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("向 Wrapper 传递凭据失败: %w", writeErr)
	}

	var readers sync.WaitGroup
	readers.Add(2)
	go func() { defer readers.Done(); m.scanWrapperOutput(stdout) }()
	go func() { defer readers.Done(); m.scanWrapperOutput(stderr) }()
	go func() {
		err := cmd.Wait()
		readers.Wait()
		m.mu.Lock()
		wasStopping := m.stopping
		if m.cmd == cmd {
			m.cmd = nil
			m.stdin = nil
			m.owned = false
			m.needs2FA = false
			m.linuxPID = 0
		}
		m.mu.Unlock()
		if err != nil && !wasStopping {
			m.log("error", "Wrapper 已退出："+redactLine(err.Error()))
		}
		m.publish()
	}()
	m.log("info", "Wrapper 已在 WSL 中启动，正在等待端口就绪")
	m.publish()
	go m.pollUntilReady()
	return nil
}

func (m *WrapperManager) SubmitTwoFactor(code string) error {
	code = strings.TrimSpace(code)
	if len(code) < 4 || len(code) > 8 {
		return fmt.Errorf("验证码格式无效")
	}
	if _, err := strconv.Atoi(code); err != nil {
		return fmt.Errorf("验证码只能包含数字")
	}
	m.mu.Lock()
	stdin := m.stdin
	if stdin == nil || !m.owned {
		m.mu.Unlock()
		return fmt.Errorf("没有等待验证码的 Wrapper 会话")
	}
	m.needs2FA = false
	m.mu.Unlock()
	value := []byte(code + "\n")
	_, err := stdin.Write(value)
	for i := range value {
		value[i] = 0
	}
	code = ""
	if err != nil {
		return fmt.Errorf("提交验证码失败: %w", err)
	}
	m.log("info", "验证码已提交，正在等待 Wrapper 就绪")
	m.publish()
	return nil
}

func (m *WrapperManager) StopOwned() error {
	m.mu.Lock()
	if !m.owned {
		m.mu.Unlock()
		return nil
	}
	m.stopping = true
	distribution := m.distribution
	pid := m.linuxPID
	cmd := m.cmd
	stdin := m.stdin
	m.mu.Unlock()

	if stdin != nil {
		_ = stdin.Close()
	}
	var stopErr error
	if pid > 0 {
		stopErr = exec.Command("wsl.exe", "-d", distribution, "--", "kill", "-TERM", strconv.Itoa(pid)).Run()
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !anyWrapperPortListening() {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if anyWrapperPortListening() && cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	m.mu.Lock()
	if m.cmd == cmd {
		m.cmd = nil
		m.stdin = nil
		m.owned = false
		m.needs2FA = false
		m.linuxPID = 0
	}
	m.mu.Unlock()
	m.log("info", "由界面启动的 Wrapper 已停止")
	m.publish()
	if stopErr != nil && anyWrapperPortListening() {
		return fmt.Errorf("Wrapper 停止后仍有端口监听，请在 WSL 中手动检查")
	}
	return nil
}

func (m *WrapperManager) scanWrapperOutput(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 16*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "AMDL_WRAPPER_PID=") {
			pid, _ := strconv.Atoi(strings.TrimPrefix(line, "AMDL_WRAPPER_PID="))
			m.mu.Lock()
			m.linuxPID = pid
			m.mu.Unlock()
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "verification") || strings.Contains(lower, "2fa") || strings.Contains(lower, "two-factor") {
			m.mu.Lock()
			m.needs2FA = true
			m.mu.Unlock()
			m.app.emit("wrapper:twofactor", true)
			m.publish()
			continue
		}
		if safe := redactLine(line); safe != "" {
			m.log("info", safe)
		}
	}
}

func (m *WrapperManager) pollUntilReady() {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		status := m.Status()
		m.app.emit("wrapper:status", status)
		if status.Ready || !status.Running || status.Needs2FA {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	m.log("warning", "Wrapper 启动超时；请检查网络、登录状态或使用外部 WSL 终端")
	m.publish()
}

func (m *WrapperManager) publish() {
	m.app.emit("wrapper:status", m.Status())
}

func (m *WrapperManager) log(level, message string) {
	m.app.emit("app:log", map[string]string{"level": level, "message": redactLine(message)})
}

func checkWrapperPorts() []PortStatus {
	result := make([]PortStatus, 0, len(wrapperPorts))
	for _, port := range wrapperPorts {
		connection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 250*time.Millisecond)
		if err == nil {
			_ = connection.Close()
		}
		result = append(result, PortStatus{Port: port, Listening: err == nil})
	}
	return result
}

func anyWrapperPortListening() bool {
	for _, status := range checkWrapperPorts() {
		if status.Listening {
			return true
		}
	}
	return false
}

func bashQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
