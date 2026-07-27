package systemproxy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	myproxyProxyScriptName = ".myproxy_proxy.sh"
	myproxyShellHookName   = ".myproxy_shell_hook.sh"
	myproxyShellMarker     = "# Source myproxy proxy settings"
)

// setupExternalShellFile 写入代理脚本，并安装 prompt 钩子以便已打开的终端自动同步。
func (p *DarwinProxy) setupExternalShellFile(proxyURL string) error {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		return fmt.Errorf("无法获取用户主目录")
	}

	proxyFile := filepath.Join(homeDir, myproxyProxyScriptName)
	if err := os.WriteFile(proxyFile, []byte(terminalProxyExportScript(proxyURL)), 0644); err != nil {
		return fmt.Errorf("写入代理配置文件失败: %v", err)
	}

	hookFile := filepath.Join(homeDir, myproxyShellHookName)
	if err := os.WriteFile(hookFile, []byte(terminalProxyPromptHookScript()), 0644); err != nil {
		return fmt.Errorf("写入 shell 钩子失败: %v", err)
	}

	return p.ensureShellHooks(homeDir, hookFile)
}

// removeExternalShellFile 清除代理脚本与 shell 钩子。
func (p *DarwinProxy) removeExternalShellFile() error {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		return nil
	}

	proxyFile := filepath.Join(homeDir, myproxyProxyScriptName)
	hookFile := filepath.Join(homeDir, myproxyShellHookName)

	// 先写 unset：已加载 precmd 的终端在下一轮 prompt 会清掉变量
	_ = os.WriteFile(proxyFile, []byte(terminalProxyUnsetScript()), 0644)

	_ = p.stripShellHooks(homeDir)
	_ = os.Remove(proxyFile)
	_ = os.Remove(hookFile)
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

// terminalProxyPromptHookScript 生成 prompt 钩子：文件变化时自动 source / 缺失时 unset。
// 这样 LProxy 开关终端代理后，已打开的终端在按回车后即可同步，无需 reset。
func terminalProxyPromptHookScript() string {
	return `# Managed by LProxy. Do not edit.
# Syncs terminal proxy env when ~/.myproxy_proxy.sh changes (precmd / PROMPT_COMMAND).

_myproxy_proxy_file="${HOME}/.myproxy_proxy.sh"

_myproxy_apply_proxy() {
  local stamp=""
  if [ -f "$_myproxy_proxy_file" ]; then
    stamp="$(stat -f %m "$_myproxy_proxy_file" 2>/dev/null || stat -c %Y "$_myproxy_proxy_file" 2>/dev/null || echo present)"
  else
    stamp="missing"
  fi
  if [ "${_MYPROXY_PROXY_STAMP-}" = "$stamp" ]; then
    return 0
  fi
  _MYPROXY_PROXY_STAMP="$stamp"
  if [ -f "$_myproxy_proxy_file" ]; then
    # shellcheck disable=SC1090
    . "$_myproxy_proxy_file"
  else
    unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy ALL_PROXY all_proxy NO_PROXY no_proxy 2>/dev/null || true
  fi
}

_myproxy_apply_proxy

if [ -n "${ZSH_VERSION-}" ]; then
  if typeset -f add-zsh-hook >/dev/null 2>&1; then
    :
  else
    autoload -Uz add-zsh-hook 2>/dev/null || true
  fi
  if typeset -f add-zsh-hook >/dev/null 2>&1; then
    add-zsh-hook -d precmd _myproxy_apply_proxy 2>/dev/null || true
    add-zsh-hook precmd _myproxy_apply_proxy
  else
    case " ${precmd_functions[*]} " in
      *" _myproxy_apply_proxy "*) ;;
      *) precmd_functions=(_myproxy_apply_proxy ${precmd_functions[@]}) ;;
    esac
  fi
elif [ -n "${BASH_VERSION-}" ]; then
  case ";${PROMPT_COMMAND-};" in
    *";_myproxy_apply_proxy;"*|*_myproxy_apply_proxy*) ;;
    *) PROMPT_COMMAND="_myproxy_apply_proxy${PROMPT_COMMAND:+;${PROMPT_COMMAND}}" ;;
  esac
fi
`
}

func (p *DarwinProxy) ensureShellHooks(homeDir, hookFile string) error {
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

	sourceLine := "source " + hookFile
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

// stripMyproxyShellHooks 移除 myproxy 写入的标记注释与 source 行（含历史遗留）。
func stripMyproxyShellHooks(content string) string {
	lines := strings.Split(content, "\n")
	newLines := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == myproxyShellMarker {
			continue
		}
		if strings.HasPrefix(trimmed, "# Source myproxy") {
			continue
		}
		if strings.Contains(line, myproxyProxyScriptName) && strings.Contains(line, "source") {
			continue
		}
		if strings.Contains(line, myproxyShellHookName) && strings.Contains(line, "source") {
			continue
		}
		newLines = append(newLines, line)
	}

	for len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) == "" {
		newLines = newLines[:len(newLines)-1]
	}
	if len(newLines) == 0 {
		return ""
	}
	return strings.Join(newLines, "\n") + "\n"
}
