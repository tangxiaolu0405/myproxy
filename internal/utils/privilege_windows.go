//go:build windows

package utils

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// IsElevated 返回当前进程是否以管理员（提升）权限运行。
func IsElevated() (bool, error) {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return false, fmt.Errorf("检查管理员权限失败: %w", err)
	}
	defer token.Close()
	return token.IsElevated(), nil
}

// RequireElevatedForTUN 在启用 TUN 前检查管理员权限。
func RequireElevatedForTUN() error {
	ok, err := IsElevated()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("TUN 全局模式需要以管理员身份运行本程序")
	}
	return nil
}
