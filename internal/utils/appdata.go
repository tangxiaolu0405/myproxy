package utils

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// AppDataFolderName 用户配置目录下的应用数据文件夹名。
	AppDataFolderName = "LProxy"
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
//  2. 当前工作目录下已有 data/myproxy.db，或存在 go.mod（开发/便携布局）→ <cwd>/data
//  3. 否则 os.UserConfigDir()/LProxy（macOS 双击 .app 时 cwd 不可靠，必须用此路径）
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

	if wd, err := os.Getwd(); err == nil {
		localData := filepath.Join(wd, "data")
		if fileExists(filepath.Join(localData, DBFileName)) {
			return localData, nil
		}
		// go run / 在仓库根目录启动：保持 ./data 布局
		if fileExists(filepath.Join(wd, "go.mod")) {
			return localData, nil
		}
	}

	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, AppDataFolderName), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
