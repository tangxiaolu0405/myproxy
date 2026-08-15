package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
)

// GenerateServerID 生成服务器唯一ID。
// 参数：
//   - addr: 服务器地址
//   - port: 服务器端口
//   - username: 用户名（用于唯一性）
//
// 返回：服务器唯一ID（MD5哈希）
//
// 注意：该 ID 必须是确定性的（同一服务器每次生成相同 ID）。
// 订阅刷新时所有服务器会删除重建，若 ID 含随机/时间因子，刷新后节点 ID 全部变化，
// 会导致链式代理（proxyChain）、选中节点等按 ID 引用的配置全部失效。
func GenerateServerID(addr string, port int, username string) string {
	// 使用地址、端口和用户名生成唯一ID（不含时间，保证同一服务器 ID 稳定）
	data := fmt.Sprintf("%s:%d:%s", addr, port, username)
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}
