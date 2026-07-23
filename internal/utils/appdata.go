package utils

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// DBFileName SQLite 数据库文件名。
	DBFileName = "myproxy.db"
	// dataDirEnv 可覆盖数据目录的环境变量。
	dataDirEnv = "MYPROXY_DATA_DIR"
)

var (
	dataDirOnce sync.Once
	dataDirPath string
	dataDirErr  error
)

// DataDir 返回应用数据目录（含数据库、默认日志、诊断导出等）。
// 解析规则（首次调用后缓存）：
//  1. 环境变量 MYPROXY_DATA_DIR
//  2. go run（可执行文件在 go-build 缓存里）→ <cwd>/data
//  3. 否则 → <启动位置>/data
//     - 双击 LProxy.app：启动位置 = .app 所在目录
//     - 直接运行二进制：启动位置 = 可执行文件所在目录
//
// 注意：macOS 双击 .app 时 os.Getwd() 常为 $HOME，不能用作数据根目录。
func DataDir() (string, error) {
	dataDirOnce.Do(func() {
		dataDirPath, dataDirErr = resolveDataDir()
		if dataDirErr != nil {
			return
		}
		dataDirErr = os.MkdirAll(dataDirPath, 0o755)
	})
	return dataDirPath, dataDirErr
}

// DBPath 返回 SQLite 数据库完整路径。
func DBPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DBFileName), nil
}

// ResolveUnderDataDir 若 path 为相对路径，则接到 DataDir 下；已是绝对路径则原样返回。
func ResolveUnderDataDir(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return DataDir()
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, path), nil
}

func resolveDataDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv(dataDirEnv)); v != "" {
		return filepath.Clean(v), nil
	}

	launchDir, err := launchDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(launchDir, "data"), nil
}

// launchDir 返回“启动位置”：放置 .app / 二进制的目录；go run 时回退到 cwd。
func launchDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return os.Getwd()
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		exe, _ = os.Executable()
	}

	dir := filepath.Dir(exe)
	// go run：二进制在临时缓存目录，用当前工作目录（通常是仓库根）
	if strings.Contains(strings.ToLower(dir), "go-build") {
		return os.Getwd()
	}

	// macOS .app：.../LProxy.app/Contents/MacOS/LProxy → 使用 .app 的父目录
	if appBundle := findAppBundleRoot(exe); appBundle != "" {
		return filepath.Dir(appBundle), nil
	}

	return dir, nil
}

// findAppBundleRoot 若 path 位于 Something.app/Contents/MacOS/ 下，返回 Something.app 路径。
func findAppBundleRoot(exePath string) string {
	const marker = ".app" + string(filepath.Separator) + "Contents" + string(filepath.Separator) + "MacOS"
	normalized := filepath.Clean(exePath)
	idx := strings.Index(strings.ToLower(normalized), strings.ToLower(marker))
	if idx < 0 {
		return ""
	}
	return normalized[:idx+len(".app")]
}
