package systemproxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DarwinProxy macOS 平台的代理实现
type DarwinProxy struct {
	proxyHost string
	proxyPort int
}

func newDarwinProxy(host string, port int) *DarwinProxy {
	return &DarwinProxy{
		proxyHost: host,
		proxyPort: port,
	}
}

// ClearSystemProxy 清除 macOS 系统代理设置
func (p *DarwinProxy) ClearSystemProxy() error {
	services, err := p.getNetworkServices()
	if err != nil {
		return fmt.Errorf("获取网络服务失败: %v", err)
	}

	for _, service := range services {
		// 清除 HTTP 代理
		cmd := exec.Command("networksetup", "-setwebproxystate", service, "off")
		_ = cmd.Run()

		// 清除 HTTPS 代理
		cmd = exec.Command("networksetup", "-setsecurewebproxystate", service, "off")
		_ = cmd.Run()

		// 清除 SOCKS 代理
		cmd = exec.Command("networksetup", "-setsocksfirewallproxystate", service, "off")
		_ = cmd.Run()
	}
	return nil
}

// SetSystemProxy 设置 macOS 系统代理
func (p *DarwinProxy) SetSystemProxy(host string, port int) error {
	services, err := p.getNetworkServices()
	if err != nil {
		return fmt.Errorf("获取网络服务失败: %v", err)
	}

	portStr := fmt.Sprintf("%d", port)
	for _, service := range services {
		// 设置 HTTP 代理
		cmd := exec.Command("networksetup", "-setwebproxy", service, host, portStr)
		if err := cmd.Run(); err != nil {
			continue
		}

		// 设置 HTTPS 代理
		cmd = exec.Command("networksetup", "-setsecurewebproxy", service, host, portStr)
		_ = cmd.Run()

		// 设置 SOCKS 代理
		cmd = exec.Command("networksetup", "-setsocksfirewallproxy", service, host, portStr)
		_ = cmd.Run()
	}
	return nil
}

// SetTerminalProxy 设置终端代理（使用外部shell文件方案）
func (p *DarwinProxy) SetTerminalProxy(host string, port int, proxyType string) error {
	proxyURL := TerminalProxyURL(host, port, proxyType)

	// 1. 设置当前进程环境变量（立即生效）
	applyTerminalProxyEnv(proxyURL)

	// 2. 使用外部shell文件方案（推荐）
	return p.setupExternalShellFile(proxyURL)
}

// ClearTerminalProxy 清除终端代理
func (p *DarwinProxy) ClearTerminalProxy() error {
	// 清除当前进程环境变量（大小写成对 unset）
	clearTerminalProxyEnv()

	// 清除外部shell文件与 rc 钩子
	return p.removeExternalShellFile()
}

// GetCurrentProxyMode 获取当前代理模式
func (p *DarwinProxy) GetCurrentProxyMode() ProxyMode {
	if os.Getenv("HTTP_PROXY") != "" || os.Getenv("http_proxy") != "" {
		return ProxyModeTerminal
	}
	return ProxyModeNone
}

// getNetworkServices 获取 macOS 网络服务列表
func (p *DarwinProxy) getNetworkServices() ([]string, error) {
	cmd := exec.Command("networksetup", "-listallnetworkservices")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	var services []string
	for i, line := range lines {
		if i == 0 {
			continue // 跳过第一行标题
		}
		line = strings.TrimSpace(line)
		if line != "" {
			services = append(services, line)
		}
	}

	if len(services) == 0 {
		return []string{"Wi-Fi", "Ethernet"}, nil // 默认服务
	}

	return services, nil
}

const (
	myproxyProxyScriptName = ".myproxy_proxy.sh"
	myproxyShellMarker     = "# Source myproxy proxy settings"
)

// setupExternalShellFile 使用外部shell文件方案设置代理
// 方案：在 ~/.myproxy_proxy.sh 中定义代理环境变量，然后在 shell 配置文件中 source 它
func (p *DarwinProxy) setupExternalShellFile(proxyURL string) error {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		return fmt.Errorf("无法获取用户主目录")
	}

	// 1. 创建外部代理配置文件
	proxyFile := filepath.Join(homeDir, myproxyProxyScriptName)
	if err := os.WriteFile(proxyFile, []byte(terminalProxyExportScript(proxyURL)), 0644); err != nil {
		return fmt.Errorf("写入代理配置文件失败: %v", err)
	}

	// 2. 在常见 shell rc 中确保存在唯一 source 钩子
	return p.ensureShellHooks(homeDir, proxyFile)
}

// removeExternalShellFile 移除外部shell文件配置
func (p *DarwinProxy) removeExternalShellFile() error {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		return nil
	}

	proxyFile := filepath.Join(homeDir, myproxyProxyScriptName)

	// 先写成 unset 脚本：若 rc 里仍有残留 source，新开终端也会清变量
	_ = os.WriteFile(proxyFile, []byte(terminalProxyUnsetScript()), 0644)

	// 从所有常见 rc 中移除钩子（不依赖当前 GUI 进程的 $SHELL）
	_ = p.stripShellHooks(homeDir)

	// 最后删除脚本文件
	_ = os.Remove(proxyFile)
	return nil
}

func applyTerminalProxyEnv(proxyURL string) {
	for _, key := range terminalProxyEnvVars {
		if key == "NO_PROXY" || key == "no_proxy" {
			continue
		}
		_ = os.Setenv(key, proxyURL)
	}
}

func clearTerminalProxyEnv() {
	for _, key := range terminalProxyEnvVars {
		_ = os.Unsetenv(key)
	}
}

func shellRCCandidates(homeDir string) []string {
	return []string{
		filepath.Join(homeDir, ".zshrc"),
		filepath.Join(homeDir, ".zprofile"),
		filepath.Join(homeDir, ".bashrc"),
		filepath.Join(homeDir, ".bash_profile"),
		filepath.Join(homeDir, ".profile"),
	}
}

func (p *DarwinProxy) ensureShellHooks(homeDir, proxyFile string) error {
	// 先清全部，再只写入用户当前 SHELL 对应的主 rc，避免多处 source
	_ = p.stripShellHooks(homeDir)

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	configFile := filepath.Join(homeDir, ".zshrc")
	if strings.Contains(shell, "bash") {
		configFile = filepath.Join(homeDir, ".bashrc")
	}

	content, err := os.ReadFile(configFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取 shell 配置失败: %v", err)
	}

	sourceLine := "source " + proxyFile
	cleaned := stripMyproxyShellHooks(string(content))
	newContent := cleaned
	if len(newContent) > 0 && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += myproxyShellMarker + "\n" + sourceLine + "\n"

	if newContent == string(content) {
		return nil
	}
	return os.WriteFile(configFile, []byte(newContent), 0644)
}

func (p *DarwinProxy) stripShellHooks(homeDir string) error {
	var firstErr error
	for _, configFile := range shellRCCandidates(homeDir) {
		content, err := os.ReadFile(configFile)
		if err != nil {
			continue
		}
		contentStr := string(content)
		newContent := stripMyproxyShellHooks(contentStr)
		if newContent == contentStr {
			continue
		}
		if err := os.WriteFile(configFile, []byte(newContent), 0644); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// stripMyproxyShellHooks 移除 myproxy 写入的标记注释与 source 行（含历史遗留的孤立注释）。
func stripMyproxyShellHooks(content string) string {
	lines := strings.Split(content, "\n")
	newLines := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == myproxyShellMarker {
			continue
		}
		// 兼容旧注释变体
		if strings.HasPrefix(trimmed, "# Source myproxy") {
			continue
		}
		if strings.Contains(line, myproxyProxyScriptName) && strings.Contains(line, "source") {
			continue
		}
		newLines = append(newLines, line)
	}

	// 去掉因删除产生的文件末尾多余空行
	for len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) == "" {
		newLines = newLines[:len(newLines)-1]
	}
	if len(newLines) == 0 {
		return ""
	}
	return strings.Join(newLines, "\n") + "\n"
}
