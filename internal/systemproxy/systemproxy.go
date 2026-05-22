package systemproxy

import "sync"

// ProxyMode 代理模式
type ProxyMode string

const (
	// ProxyModeNone 清除系统代理
	ProxyModeNone ProxyMode = "none"
	// ProxyModeAuto 自动配置系统代理
	ProxyModeAuto ProxyMode = "auto"
	// ProxyModeTerminal 命令行终端代理（环境变量代理）
	ProxyModeTerminal ProxyMode = "terminal"
)

// SystemProxy 系统代理管理器
// 使用策略模式，根据平台自动选择对应的实现
type SystemProxy struct {
	mu        sync.Mutex
	platform  PlatformProxy
	proxyHost string
	proxyPort int
}

// NewSystemProxy 创建系统代理管理器
// 根据当前运行平台自动选择对应的实现
func NewSystemProxy(proxyHost string, proxyPort int) *SystemProxy {
	return &SystemProxy{
		platform:  NewPlatformProxy(proxyHost, proxyPort),
		proxyHost: proxyHost,
		proxyPort: proxyPort,
	}
}

// ClearSystemProxy 清除系统代理设置
func (sp *SystemProxy) ClearSystemProxy() error {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return sp.platform.ClearSystemProxy()
}

// SetSystemProxy 自动配置系统代理
func (sp *SystemProxy) SetSystemProxy() error {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return sp.platform.SetSystemProxy(sp.proxyHost, sp.proxyPort)
}

// SetTerminalProxy 设置终端代理（环境变量代理）
func (sp *SystemProxy) SetTerminalProxy(proxyType string) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return sp.platform.SetTerminalProxy(sp.proxyHost, sp.proxyPort, proxyType)
}

// ClearTerminalProxy 清除终端代理设置
func (sp *SystemProxy) ClearTerminalProxy() error {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return sp.platform.ClearTerminalProxy()
}

// SetGitProxy 写入 git config --global 的 http/https.proxy（与终端代理使用同一套 URL 规则）。
func (sp *SystemProxy) SetGitProxy(proxyType string) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return SetGitGlobalProxy(sp.proxyHost, sp.proxyPort, proxyType)
}

// ClearGitProxy 清除 git config --global 的 http/https.proxy。
func (sp *SystemProxy) ClearGitProxy() error {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return ClearGitGlobalProxy()
}

// GetCurrentProxyMode 获取当前代理模式
func (sp *SystemProxy) GetCurrentProxyMode() ProxyMode {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return sp.platform.GetCurrentProxyMode()
}

// UpdateProxy 更新代理地址和端口（用于动态更新）
func (sp *SystemProxy) UpdateProxy(host string, port int) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.proxyHost = host
	sp.proxyPort = port
	sp.platform = NewPlatformProxy(host, port)
}
