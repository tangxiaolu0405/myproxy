package systemproxy

import (
	"fmt"
	"os"
	"os/exec"
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
		cmd := exec.Command("networksetup", "-setwebproxystate", service, "off")
		_ = cmd.Run()

		cmd = exec.Command("networksetup", "-setsecurewebproxystate", service, "off")
		_ = cmd.Run()

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
		cmd := exec.Command("networksetup", "-setwebproxy", service, host, portStr)
		if err := cmd.Run(); err != nil {
			continue
		}

		cmd = exec.Command("networksetup", "-setsecurewebproxy", service, host, portStr)
		_ = cmd.Run()

		cmd = exec.Command("networksetup", "-setsocksfirewallproxy", service, host, portStr)
		_ = cmd.Run()
	}
	return nil
}

// SetTerminalProxy 设置终端代理（外部脚本 + prompt 钩子，已打开终端回车后自动同步）
func (p *DarwinProxy) SetTerminalProxy(host string, port int, proxyType string) error {
	proxyURL := TerminalProxyURL(host, port, proxyType)
	applyTerminalProxyEnv(proxyURL)
	return p.setupExternalShellFile(proxyURL)
}

// ClearTerminalProxy 清除终端代理
func (p *DarwinProxy) ClearTerminalProxy() error {
	clearTerminalProxyEnv()
	return p.removeExternalShellFile()
}

// GetCurrentProxyMode 获取当前代理模式
func (p *DarwinProxy) GetCurrentProxyMode() ProxyMode {
	if os.Getenv("HTTP_PROXY") != "" || os.Getenv("http_proxy") != "" {
		return ProxyModeTerminal
	}
	return ProxyModeNone
}

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
			continue
		}
		line = strings.TrimSpace(line)
		if line != "" {
			services = append(services, line)
		}
	}

	if len(services) == 0 {
		return []string{"Wi-Fi", "Ethernet"}, nil
	}

	return services, nil
}
