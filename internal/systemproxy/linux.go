package systemproxy

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// LinuxProxy Linux 平台的代理实现（优先 GNOME gsettings，其次 KDE，再回退为仅进程环境变量）。
type LinuxProxy struct {
	proxyHost string
	proxyPort int
}

func newLinuxProxy(host string, port int) *LinuxProxy {
	return &LinuxProxy{
		proxyHost: host,
		proxyPort: port,
	}
}

func (p *LinuxProxy) ClearSystemProxy() error {
	desktop := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP"))
	var errs []string
	if strings.Contains(desktop, "gnome") || strings.Contains(desktop, "unity") || strings.Contains(desktop, "cinnamon") || hasCommand("gsettings") {
		if err := clearGNOMEProxy(); err != nil {
			errs = append(errs, err.Error())
		} else {
			return nil
		}
	}
	if strings.Contains(desktop, "kde") || hasCommand("kwriteconfig5") || hasCommand("kwriteconfig6") {
		if err := clearKDEProxy(); err != nil {
			errs = append(errs, err.Error())
		} else {
			return nil
		}
	}
	if len(errs) == 0 {
		// 无桌面代理工具时视为成功（终端代理仍可用）
		return nil
	}
	return fmt.Errorf("清除 Linux 系统代理失败: %s", strings.Join(errs, "; "))
}

func (p *LinuxProxy) SetSystemProxy(host string, port int) error {
	desktop := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP"))
	var errs []string
	if strings.Contains(desktop, "gnome") || strings.Contains(desktop, "unity") || strings.Contains(desktop, "cinnamon") || hasCommand("gsettings") {
		if err := setGNOMEProxy(host, port); err != nil {
			errs = append(errs, err.Error())
		} else {
			return nil
		}
	}
	if strings.Contains(desktop, "kde") || hasCommand("kwriteconfig5") || hasCommand("kwriteconfig6") {
		if err := setKDEProxy(host, port); err != nil {
			errs = append(errs, err.Error())
		} else {
			return nil
		}
	}
	if len(errs) == 0 {
		return fmt.Errorf("未检测到可用的桌面代理配置工具（gsettings/kwriteconfig），请使用终端代理")
	}
	return fmt.Errorf("设置 Linux 系统代理失败: %s", strings.Join(errs, "; "))
}

func (p *LinuxProxy) SetTerminalProxy(host string, port int, proxyType string) error {
	proxyURL := TerminalProxyURL(host, port, proxyType)
	os.Setenv("HTTP_PROXY", proxyURL)
	os.Setenv("HTTPS_PROXY", proxyURL)
	os.Setenv("http_proxy", proxyURL)
	os.Setenv("https_proxy", proxyURL)
	os.Setenv("ALL_PROXY", proxyURL)
	os.Setenv("all_proxy", proxyURL)
	return nil
}

func (p *LinuxProxy) ClearTerminalProxy() error {
	os.Unsetenv("HTTP_PROXY")
	os.Unsetenv("HTTPS_PROXY")
	os.Unsetenv("http_proxy")
	os.Unsetenv("https_proxy")
	os.Unsetenv("ALL_PROXY")
	os.Unsetenv("all_proxy")
	return nil
}

func (p *LinuxProxy) GetCurrentProxyMode() ProxyMode {
	if mode, err := getGNOMEProxyMode(); err == nil && mode == "manual" {
		return ProxyModeAuto
	}
	if os.Getenv("HTTP_PROXY") != "" || os.Getenv("http_proxy") != "" {
		return ProxyModeTerminal
	}
	return ProxyModeNone
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %v (%s)", name, args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func setGNOMEProxy(host string, port int) error {
	portStr := strconv.Itoa(port)
	steps := [][]string{
		{"gsettings", "set", "org.gnome.system.proxy", "mode", "manual"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "host", host},
		{"gsettings", "set", "org.gnome.system.proxy.http", "port", portStr},
		{"gsettings", "set", "org.gnome.system.proxy.https", "host", host},
		{"gsettings", "set", "org.gnome.system.proxy.https", "port", portStr},
		{"gsettings", "set", "org.gnome.system.proxy.socks", "host", host},
		{"gsettings", "set", "org.gnome.system.proxy.socks", "port", portStr},
	}
	for _, args := range steps {
		if err := runCmd(args[0], args[1:]...); err != nil {
			return err
		}
	}
	return nil
}

func clearGNOMEProxy() error {
	return runCmd("gsettings", "set", "org.gnome.system.proxy", "mode", "none")
}

func getGNOMEProxyMode() (string, error) {
	cmd := exec.Command("gsettings", "get", "org.gnome.system.proxy", "mode")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.Trim(strings.TrimSpace(string(out)), "'\""), nil
}

func kdeWriteTool() string {
	if hasCommand("kwriteconfig6") {
		return "kwriteconfig6"
	}
	if hasCommand("kwriteconfig5") {
		return "kwriteconfig5"
	}
	return ""
}

func setKDEProxy(host string, port int) error {
	tool := kdeWriteTool()
	if tool == "" {
		return fmt.Errorf("kwriteconfig 不可用")
	}
	portStr := strconv.Itoa(port)
	proxyURL := fmt.Sprintf("http://%s:%s", host, portStr)
	socksURL := fmt.Sprintf("socks://%s:%s", host, portStr)
	steps := [][]string{
		{tool, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "ProxyType", "1"},
		{tool, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "httpProxy", proxyURL},
		{tool, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "httpsProxy", proxyURL},
		{tool, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "socksProxy", socksURL},
	}
	for _, args := range steps {
		if err := runCmd(args[0], args[1:]...); err != nil {
			return err
		}
	}
	return nil
}

func clearKDEProxy() error {
	tool := kdeWriteTool()
	if tool == "" {
		return fmt.Errorf("kwriteconfig 不可用")
	}
	return runCmd(tool, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "ProxyType", "0")
}
