package systemproxy

// terminalProxyEnvVars 终端代理相关环境变量（大小写均覆盖，避免 curl/git/wget 各认一套）。
var terminalProxyEnvVars = []string{
	"HTTP_PROXY", "http_proxy",
	"HTTPS_PROXY", "https_proxy",
	"ALL_PROXY", "all_proxy",
	"NO_PROXY", "no_proxy",
}

// terminalProxyExportScript 生成设置代理的 shell 脚本内容。
func terminalProxyExportScript(proxyURL string) string {
	return "# Proxy settings (set by myproxy)\n" +
		"# This file is managed by myproxy. Do not edit manually.\n\n" +
		"export HTTP_PROXY=" + proxyURL + "\n" +
		"export HTTPS_PROXY=" + proxyURL + "\n" +
		"export http_proxy=" + proxyURL + "\n" +
		"export https_proxy=" + proxyURL + "\n" +
		"export ALL_PROXY=" + proxyURL + "\n" +
		"export all_proxy=" + proxyURL + "\n"
}

// terminalProxyUnsetScript 生成清除代理的 shell 脚本内容（供残留 source 时仍能清干净）。
func terminalProxyUnsetScript() string {
	return "# Proxy settings cleared by myproxy\n" +
		"# This file is managed by myproxy. Do not edit manually.\n\n" +
		"unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy ALL_PROXY all_proxy NO_PROXY no_proxy\n"
}
