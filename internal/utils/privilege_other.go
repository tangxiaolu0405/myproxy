//go:build !windows

package utils

import (
	"fmt"
	"os"
)

// IsElevated 返回当前进程是否具备配置 TUN 所需的权限（非 Windows：通常为 root）。
func IsElevated() (bool, error) {
	return os.Geteuid() == 0, nil
}

// RequireElevatedForTUN 在启用 TUN 前检查权限。
func RequireElevatedForTUN() error {
	ok, err := IsElevated()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("TUN 全局模式需要 root 权限（例如 sudo 运行）")
	}
	return nil
}
