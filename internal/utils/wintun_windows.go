//go:build windows

package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindWintunDLL 查找 wintun.dll：优先可执行文件同目录，其次当前工作目录。
func FindWintunDLL() (string, error) {
	candidates := make([]string, 0, 4)
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "wintun.dll"))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "wintun.dll"))
		candidates = append(candidates, filepath.Join(wd, "third_party", "wintun", "wintun.dll"))
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到 wintun.dll（请放在程序同目录；构建产物应附带该文件）")
}

// EnsureWintunForTUN 启用 TUN 前确认 wintun.dll 可用。
func EnsureWintunForTUN() error {
	_, err := FindWintunDLL()
	return err
}
