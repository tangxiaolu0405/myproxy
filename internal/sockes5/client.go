package sockes5

import (
	"fmt"
	"io"
	"net"
)

// 定义 SOCKS5 客户端结构体
type SOCKS5Client struct {
	ProxyAddr string // SOCKS5 代理服务器地址 (e.g., "127.0.0.1:1080")
	Username  string // 认证用户名 (如果需要)
	Password  string // 认证密码 (如果需要)
}

// SOCKS5 协议常量
const (
	Version      = 0x05 // SOCKS5 版本号
	AuthNoAuth   = 0x00 // 无需认证
	AuthUserPass = 0x02 // 用户名/密码认证

	CmdConnect      = 0x01 // CONNECT 命令
	CmdUdpAssociate = 0x03 // UDP ASSOCIATE 命令 (新增)
	ATypIPv4        = 0x01 // 地址类型：IPv4
	ATypDomain      = 0x03 // 地址类型：域名
	ATypIPv6        = 0x04 // 地址类型：IPv6

	ReplySuccess = 0x00 // 响应：成功
)

// 代理服务器分配的 UDP 监听地址和端口
type ProxyUDPAddr struct {
	Host string
	Port int
}

// Dial 负责连接 SOCKS5 服务器并完成协商和认证
func (c *SOCKS5Client) Dial(network, addr string) (net.Conn, error) {
	// 1. 连接 SOCKS5 代理服务器
	conn, err := net.Dial("tcp", c.ProxyAddr)
	if err != nil {
		return nil, fmt.Errorf("连接代理服务器失败: %w", err)
	}

	// --- 阶段 1: 协商 ---
	if err := c.negotiate(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 协商失败: %w", err)
	}

	// --- 阶段 2: 认证 (如果需要) ---
	if c.Username != "" || c.Password != "" {
		if err := c.authenticate(conn); err != nil {
			conn.Close()
			return nil, fmt.Errorf("SOCKS5 认证失败: %w", err)
		}
	}

	// --- 阶段 3 & 4: 请求并获取响应 ---
	if err := c.sendRequest(conn, CmdConnect, addr); err != nil {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 请求失败: %w", err)
	}

	// 连接已建立，返回连接对象
	return conn, nil
}

// 阶段 1: 协商
func (c *SOCKS5Client) negotiate(conn net.Conn) error {
	// 客户端发送: VER (1) + NMETHODS (1) + METHODS (N)
	methods := []byte{AuthNoAuth} // 默认支持无认证
	if c.Username != "" || c.Password != "" {
		methods = append(methods, AuthUserPass) // 如果有用户名密码，添加支持
	}

	buf := []byte{Version, byte(len(methods))}
	buf = append(buf, methods...)

	if _, err := conn.Write(buf); err != nil {
		return err
	}

	// 服务器接收: VER (1) + METHOD (1)
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return err
	}

	if reply[0] != Version {
		return fmt.Errorf("SOCKS 版本不匹配: %d", reply[0])
	}

	// 检查服务器选择的认证方法是否支持
	if reply[1] == AuthNoAuth {
		return nil
	} else if reply[1] == AuthUserPass {
		return nil // 需要进行用户密码认证
	} else {
		return fmt.Errorf("服务器选择了不支持的认证方法: %d", reply[1])
	}
}

// 阶段 2: 用户名/密码认证 (仅在服务器选择 AuthUserPass 时调用)
func (c *SOCKS5Client) authenticate(conn net.Conn) error {
	// 客户端发送: U_VER (1) + ULEN (1) + UNAME (ULEN) + PLEN (1) + PASSWD (PLEN)

	// 认证协议版本固定为 0x01
	authReq := []byte{0x01}

	// 用户名
	authReq = append(authReq, byte(len(c.Username)))
	authReq = append(authReq, []byte(c.Username)...)

	// 密码
	authReq = append(authReq, byte(len(c.Password)))
	authReq = append(authReq, []byte(c.Password)...)

	if _, err := conn.Write(authReq); err != nil {
		return err
	}

	// 服务器接收: U_VER (1) + STATUS (1)
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return err
	}

	if reply[0] != 0x01 {
		return fmt.Errorf("认证响应版本错误: %d", reply[0])
	}
	if reply[1] != ReplySuccess {
		return fmt.Errorf("认证失败，状态码: %d", reply[1])
	}

	return nil
}

// 阶段 3 & 4: 发送连接请求，并解析响应
func (c *SOCKS5Client) sendRequest(conn net.Conn, cmd byte, addr string) error {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("地址格式错误: %w", err)
	}
	port, _ := net.LookupPort("tcp", portStr)

	// 构建请求包: VER (1) + CMD (1) + RSV (1) + ATYP (1) + DST.ADDR (N) + DST.PORT (2)
	req := []byte{Version, cmd, 0x00}

	// 地址类型：使用域名
	req = append(req, ATypDomain)
	req = append(req, byte(len(host)))
	req = append(req, []byte(host)...)

	// 端口 (使用大端序)
	req = append(req, byte(port>>8), byte(port&0xff))

	if _, err := conn.Write(req); err != nil {
		return err
	}

	// 解析响应: VER (1) + REP (1) + RSV (1) + ATYP (1) + BND.ADDR (N) + BND.PORT (2)
	reply := make([]byte, 4) // 读取前 4 字节
	if _, err := io.ReadFull(conn, reply); err != nil {
		return err
	}

	if reply[0] != Version {
		return fmt.Errorf("SOCKS 版本不匹配: %d", reply[0])
	}
	if reply[1] != ReplySuccess {
		return fmt.Errorf("代理请求被拒绝，状态码: %d", reply[1])
	}

	// 接下来需要根据 ATYP 读取绑定地址和端口，但对于 CONNECT 成功而言，通常可以跳过（仅丢弃剩余数据）
	// 为了简化，我们只读取绑定地址和端口的头部，实际项目中需要完整解析。

	var addrLen int
	switch reply[3] {
	case ATypIPv4:
		addrLen = 4 // IPv4 地址 4 字节
	case ATypDomain:
		lenBuf := make([]byte, 1) // 域名长度 1 字节
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return err
		}
		addrLen = int(lenBuf[0]) // 域名长度
	case ATypIPv6:
		addrLen = 16 // IPv6 地址 16 字节
	default:
		return fmt.Errorf("不支持的地址类型: %d", reply[3])
	}

	// 丢弃 BND.ADDR + BND.PORT
	discardLen := addrLen + 2
	if reply[3] == ATypDomain {
		discardLen++ // 域名长度字节已读，但为了通用性，需要调整。
	}

	discard := make([]byte, discardLen)
	_, _ = io.ReadFull(conn, discard) // 简单读取并丢弃，不检查错误

	return nil
}

// UDPAssociate: 执行 SOCKS5 UDP 关联流程，返回代理服务器的 UDP 监听地址
// 它保持 TCP 连接打开，直到 UDP 会话结束。
func (c *SOCKS5Client) UDPAssociate() (net.Conn, ProxyUDPAddr, error) {
    // 1. 连接 SOCKS5 代理服务器 (TCP)
    conn, err := net.Dial("tcp", c.ProxyAddr)
    if err != nil {
        return nil, ProxyUDPAddr{}, fmt.Errorf("连接代理服务器失败: %w", err)
    }

    // --- 阶段 1: 协商 ---
    if err := c.negotiate(conn); err != nil {
        conn.Close()
        return nil, ProxyUDPAddr{}, fmt.Errorf("SOCKS5 协商失败: %w", err)
    }

    // --- 阶段 2: 认证 (如果需要) ---
    // 假设 authenticate 方法已经存在，逻辑与 TCP Connect 相同
    if c.Username != "" || c.Password != "" {
        if err := c.authenticate(conn); err != nil {
            conn.Close()
            return nil, ProxyUDPAddr{}, fmt.Errorf("SOCKS5 认证失败: %w", err)
        }
    }

    // --- 阶段 3: 发送 UDP ASSOCIATE 请求 ---
    // 目标地址设置为 0.0.0.0:0，表示客户端不关心绑定地址，由代理服务器决定
    req := []byte{Version, CmdUdpAssociate, 0x00, ATypIPv4, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
    if _, err := conn.Write(req); err != nil {
        conn.Close()
        return nil, ProxyUDPAddr{}, fmt.Errorf("发送 UDP ASSOCIATE 请求失败: %w", err)
    }

    // --- 阶段 4: 解析响应 (获取代理服务器的 UDP 监听地址) ---
    proxyUDPAddr, err := c.parseUDPReply(conn)
    if err != nil {
        conn.Close()
        return nil, ProxyUDPAddr{}, fmt.Errorf("解析 UDP 响应失败: %w", err)
    }

    // 返回打开的 TCP 连接（必须保持活跃）和代理的 UDP 地址
    return conn, proxyUDPAddr, nil
}

// 解析 SOCKS5 响应，提取代理服务器的 UDP 绑定地址
func (c *SOCKS5Client) parseUDPReply(conn net.Conn) (ProxyUDPAddr, error) {
    // 响应格式: VER (1) + REP (1) + RSV (1) + ATYP (1) + BND.ADDR (N) + BND.PORT (2)
    reply := make([]byte, 4) // 读取前 4 字节
    if _, err := io.ReadFull(conn, reply); err != nil {
        return ProxyUDPAddr{}, err
    }
    
    if reply[0] != Version {
        return ProxyUDPAddr{}, fmt.Errorf("SOCKS 版本不匹配: %d", reply[0])
    }
    if reply[1] != ReplySuccess {
        return ProxyUDPAddr{}, fmt.Errorf("UDP 关联被拒绝，状态码: %d", reply[1])
    }
    
    atyp := reply[3]
    var addr string
    var addrLen int

    // 根据 ATYP 确定地址的长度
    switch atyp {
    case ATypIPv4:
        addrLen = 4
    case ATypDomain:
        lenBuf := make([]byte, 1) // 读取域名长度
        if _, err := io.ReadFull(conn, lenBuf); err != nil {
            return ProxyUDPAddr{}, err
        }
        addrLen = int(lenBuf[0])
    default:
        return ProxyUDPAddr{}, fmt.Errorf("不支持的地址类型: %d", atyp)
    }
    
    // 读取地址和端口
    payload := make([]byte, addrLen+2) // 地址 + 端口 (2 bytes)
    if _, err := io.ReadFull(conn, payload); err != nil {
        return ProxyUDPAddr{}, err
    }
    
    // 提取地址
    if atyp == ATypIPv4 {
        addr = net.IP(payload[:addrLen]).String()
    } else if atyp == ATypDomain {
        addr = string(payload[:addrLen])
    }

    // 提取端口 (大端序)
    portBytes := payload[addrLen:]
    port := int(portBytes[0])<<8 | int(portBytes[1])

    return ProxyUDPAddr{Host: addr, Port: port}, nil
}
