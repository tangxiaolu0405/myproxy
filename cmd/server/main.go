package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

const (
	Version      = 0x05 // SOCKS5 版本
	AuthNoAuth   = 0x00 // 无认证
	CmdConnect   = 0x01 // CONNECT 命令
	ATypIPv4     = 0x01 // 地址类型：IPv4
	ATypDomain   = 0x03 // 地址类型：域名
	ReplySuccess = 0x00 // 响应：成功
)

func main() {
	// 监听本地 1080 端口
	listener, err := net.Listen("tcp", "0.0.0.0:1080")
	if err != nil {
		log.Fatalf("启动 SOCKS5 服务器失败: %v", err)
	}
	log.Printf("SOCKS5 服务器已启动，监听 0.0.0.0:1080 (无认证)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("接受连接失败: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	// 1. 协商 (Negotiation)
	if err := doNegotiation(conn); err != nil {
		log.Printf("协商失败: %v", err)
		return
	}

	// 2. 请求 (Request)
	targetConn, err := handleRequest(conn)
	if err != nil {
		log.Printf("请求处理失败: %v", err)
		return
	}
	defer targetConn.Close()

	// 3. 读取请求头部 (VER, CMD, RSV, ATYP)
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		log.Printf("读取请求头部失败: %v", err)
		return
	}
	
	cmd := buf[1] // 获取命令类型

	if cmd == CmdConnect {
		// --- TCP 连接逻辑 (保持不变) ---
		targetConn, err := handleConnectRequest(conn, buf) // 假设这是你处理 CONNECT 的函数
		if err != nil {
			log.Printf("CONNECT 请求处理失败: %v", err)
			return
		}
		defer targetConn.Close()
		log.Printf("开始转发 TCP 数据...")
		go io.Copy(targetConn, conn) 
		io.Copy(conn, targetConn)    
	
	} else if cmd == CmdUdpAssociate {
		// --- UDP 关联逻辑 (新增) ---
		log.Println("收到 UDP ASSOCIATE 请求")
		if err := handleUDPAssociate(conn); err != nil {
			log.Printf("UDP ASSOCIATE 处理失败: %v", err)
			return
		}
		// handleUDPAssociate 函数内部会阻塞直到 TCP 连接关闭
		
	} else {
		// 不支持的其他命令
		conn.Write([]byte{Version, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // 命令不支持
		log.Printf("不支持的 SOCKS 命令: %d", cmd)
	}
}

// 阶段 1: 协商 - 只支持无认证
func doNegotiation(conn net.Conn) error {
	// 客户端发送: VER (1) + NMETHODS (1) + METHODS (N)
	buf := make([]byte, 258) // 缓冲区足够大

	// 读取版本号和方法数量
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return fmt.Errorf("读取协商头部失败: %w", err)
	}
	if buf[0] != Version {
		return fmt.Errorf("不支持的 SOCKS 版本: %d", buf[0])
	}

	numMethods := int(buf[1])
	if _, err := io.ReadFull(conn, buf[:numMethods]); err != nil {
		return fmt.Errorf("读取方法列表失败: %w", err)
	}

	// 服务器响应: VER (1) + METHOD (1)
	// 因为我们是简易服务器，强制选择无认证 (0x00)
	_, err := conn.Write([]byte{Version, AuthNoAuth})
	return err
}

// 阶段 2 & 3: 处理请求并建立连接
func handleRequest(conn net.Conn) (net.Conn, error) {
	// 读取请求头部: VER (1) + CMD (1) + RSV (1) + ATYP (1)
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, fmt.Errorf("读取请求头部失败: %w", err)
	}

	if buf[1] != CmdConnect {
		// 响应客户端不支持的命令 (0x07)
		conn.Write([]byte{Version, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return nil, fmt.Errorf("不支持的命令: %d", buf[1])
	}

	targetAddr, err := readAddrPort(conn, buf[3])
	if err != nil {
		conn.Write([]byte{Version, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // 地址类型不支持
		return nil, err
	}

	// 尝试连接目标
	log.Printf("尝试连接目标: %s", targetAddr)
	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		// 连接目标失败 (0x05)
		conn.Write([]byte{Version, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return nil, fmt.Errorf("连接目标 %s 失败: %w", targetAddr, err)
	}

	// 响应成功: VER (1) + REP (1) + RSV (1) + ATYP (1) + BND.ADDR (N) + BND.PORT (2)
	// 绑定地址和端口使用 0.0.0.0:0 简化
	successReply := []byte{Version, ReplySuccess, 0x00, ATypIPv4, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	conn.Write(successReply)

	return targetConn, nil
}

// 读取 SOCKS5 地址和端口
func readAddrPort(conn net.Conn, atyp byte) (string, error) {
	switch atyp {
	case ATypIPv4:
		// IPv4 地址 (4 bytes) + Port (2 bytes)
		addrPort := make([]byte, 6)
		if _, err := io.ReadFull(conn, addrPort); err != nil {
			return "", err
		}
		ip := net.IP(addrPort[:4]).String()
		port := int(addrPort[4])<<8 | int(addrPort[5])
		return fmt.Sprintf("%s:%d", ip, port), nil

	case ATypDomain:
		// 域名长度 (1 byte) + Domain (N bytes) + Port (2 bytes)
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", err
		}
		domainLen := int(lenBuf[0])

		addrPort := make([]byte, domainLen+2)
		if _, err := io.ReadFull(conn, addrPort); err != nil {
			return "", err
		}
		domain := string(addrPort[:domainLen])
		port := int(addrPort[domainLen])<<8 | int(addrPort[domainLen+1])
		return fmt.Sprintf("%s:%d", domain, port), nil

	default:
		return "", fmt.Errorf("不支持的地址类型: %d", atyp)
	}
}

// handleUDPAssociate 负责处理 UDP 关联请求并启动 UDP 转发
// conn: 客户端发来 UDP ASSOCIATE 请求的 TCP 连接
func handleUDPAssociate(conn net.Conn) error {
	// 1. 在代理服务器上创建一个临时的 UDP 监听端口
	// 使用 net.ListenPacket 监听一个随机端口 (端口号 0 表示系统自动分配)
	udpAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	if err != nil {
		return err
	}
	pc, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("启动 UDP 监听失败: %w", err)
	}
	defer pc.Close() // 确保在 TCP 连接关闭时，UDP 监听也关闭

	// 获取 UDP 实际绑定的 IP 和端口
	proxyUDPAddr := pc.LocalAddr().(*net.UDPAddr)
	log.Printf("为客户端分配 UDP 监听端口: %s", proxyUDPAddr.String())

	// 2. 发送成功响应给客户端 (通过 TCP)
	// 响应格式: VER(1) + REP(1) + RSV(1) + ATYP(1) + BND.ADDR(N) + BND.PORT(2)

	// 假设代理服务器的外部 IP 是 127.0.0.1 (简化处理，实际应获取外部IP)
	boundIP := net.ParseIP("127.0.0.1")

	resp := []byte{Version, ReplySuccess, 0x00, ATypIPv4} // 0x01 = IPv4 ATYP
	resp = append(resp, boundIP.To4()...)                 // 4 字节 IP

	port := uint16(proxyUDPAddr.Port)
	resp = append(resp, byte(port>>8), byte(port&0xff)) // 2 字节端口 (大端序)

	if _, err := conn.Write(resp); err != nil {
		return fmt.Errorf("发送 UDP 关联响应失败: %w", err)
	}

	// 3. 启动 UDP 转发循环
	// 必须在单独的 goroutine 中保持 TCP 连接打开，并处理 UDP 转发

	// UDP 转发器需要知道客户端的地址，以便将响应发回
	// 我们用一个 map 来保存目标地址和其对应的远程连接，以避免每次都重新解析
	relayMap := make(map[string]*net.UDPConn) // [目标地址:端口] -> *net.UDPConn

	// 启动一个 goroutine 专门处理 UDP 数据
	go udpRelayLoop(pc, relayMap)

	// 阻塞 TCP 连接，直到客户端或超时关闭它
	// 只要 TCP 连接不关闭，UDP 转发就保持活跃
	log.Println("UDP 关联成功，等待 TCP 连接关闭以结束转发...")

	// 简单阻塞：等待直到 conn 被关闭（例如，客户端程序退出）
	// 设置一个较长的超时，防止连接无限期占用资源
	conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	buffer := make([]byte, 1)
	for {
		// 循环读取 TCP 连接，看客户端是否发送了数据（虽然不应该）或关闭连接
		_, err := conn.Read(buffer)
		if err == io.EOF {
			log.Printf("客户端关闭了 TCP 连接，结束 UDP 转发.")
			break
		}
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			log.Printf("UDP ASSOCIATE TCP 连接超时，结束转发.")
			break
		}
		if err != nil {
			log.Printf("TCP 连接错误，结束 UDP 转发: %v", err)
			break
		}
	}

	// TCP 连接关闭，UDP 端口将在 defer pc.Close() 中关闭
	return nil
}

// udpRelayLoop 负责从 UDP 监听器接收数据，解析头部，转发给目标，并将目标响应发回
// pc: 代理服务器的 UDP 监听器
// relayMap: 用于缓存目标服务器的 UDP 连接
func udpRelayLoop(pc *net.UDPConn, relayMap map[string]*net.UDPConn) {
	defer log.Println("UDP 转发循环退出。")

	// 缓冲区用于接收 SOCKS5 封装的 UDP 数据包
	buf := make([]byte, 65535) // UDP 最大尺寸

	for {
		n, clientAddr, err := pc.ReadFromUDP(buf)
		if err != nil {
			// 如果 UDP 监听器关闭 (来自 defer pc.Close())，会收到错误，循环退出
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("UDP ReadFromUDP 错误: %v", err)
			}
			return
		}

		// 忽略太短的数据包
		if n < 10 {
			continue
		}

		// 1. 解析 SOCKS5 UDP 数据报头部
		// 头部: RSV(2) + FRAG(1) + ATYP(1) + DST.ADDR(N) + DST.PORT(2) + DATA

		// 检查 RSV (0x0000) 和 FRAG (0x00)
		if buf[0] != 0x00 || buf[1] != 0x00 || buf[2] != 0x00 {
			log.Printf("收到非法的 SOCKS5 UDP 头部 (RSV/FRAG): %x", buf[:3])
			continue
		}

		atyp := buf[3]
		reader := bytes.NewReader(buf[4:n])

		targetAddr, payload, err := parseSocks5UdpHeader(reader, atyp)
		if err != nil {
			log.Printf("解析 SOCKS5 UDP 头部失败: %v", err)
			continue
		}

		// 2. 获取或创建到目标服务器的连接
		targetConn, ok := relayMap[targetAddr]
		if !ok {
			// 如果是新的目标地址，建立一个新的 UDP 连接 (用于发送数据)
			resolvedAddr, err := net.ResolveUDPAddr("udp", targetAddr)
			if err != nil {
				log.Printf("解析目标地址 %s 失败: %v", targetAddr, err)
				continue
			}
			targetConn, err = net.DialUDP("udp", nil, resolvedAddr)
			if err != nil {
				log.Printf("连接目标 UDP 服务器 %s 失败: %v", targetAddr, err)
				continue
			}
			relayMap[targetAddr] = targetConn

			// 为这个新的目标启动一个 goroutine，专门处理目标返回的响应
			go handleTargetResponse(pc, clientAddr, targetConn)
		}

		// 3. 转发数据到目标服务器
		if _, err := targetConn.Write(payload); err != nil {
			log.Printf("转发数据到目标 %s 失败: %v", targetAddr, err)
		}
	}
}

// parseSocks5UdpHeader 从 UDP 包中解析出目标地址和实际负载
func parseSocks5UdpHeader(reader *bytes.Reader, atyp byte) (string, []byte, error) {
	// ... (此函数需要实现根据 ATYP 解析地址和端口的逻辑)
	// 逻辑与客户端的解析非常相似，但它是从 Reader 中读取：

	var addrLen int
	var addr string

	switch atyp {
	case ATypIPv4:
		addrLen = 4
		ipBytes := make([]byte, 4)
		if _, err := reader.Read(ipBytes); err != nil {
			return "", nil, err
		}
		addr = net.IP(ipBytes).String()
	case ATypDomain:
		lenBuf := make([]byte, 1)
		if _, err := reader.Read(lenBuf); err != nil {
			return "", nil, err
		}
		addrLen = int(lenBuf[0])
		domainBytes := make([]byte, addrLen)
		if _, err := reader.Read(domainBytes); err != nil {
			return "", nil, err
		}
		addr = string(domainBytes)
	default:
		return "", nil, fmt.Errorf("不支持的地址类型: %d", atyp)
	}

	portBytes := make([]byte, 2)
	if _, err := reader.Read(portBytes); err != nil {
		return "", nil, err
	}
	port := int(portBytes[0])<<8 | int(portBytes[1])

	// 实际的负载数据
	payload := make([]byte, reader.Len())
	if _, err := reader.Read(payload); err != nil {
		return "", nil, err
	}

	return fmt.Sprintf("%s:%d", addr, port), payload, nil
}

// handleTargetResponse 监听目标服务器的响应，并封装发回给 SOCKS5 客户端
// pc: 代理服务器的 UDP 监听器
// clientAddr: SOCKS5 客户端的 UDP 地址 (用于定向发送)
// targetConn: 到目标服务器的 UDP 连接
func handleTargetResponse(pc *net.UDPConn, clientAddr *net.UDPAddr, targetConn *net.UDPConn) {
	defer targetConn.Close()

	// 缓冲区用于接收目标服务器的响应
	respBuf := make([]byte, 65535)

	for {
		n, targetServerAddr, err := targetConn.ReadFromUDP(respBuf)
		if err != nil {
			// 连接关闭或错误，退出循环
			return
		}

		// 1. 封装 SOCKS5 UDP 响应头部
		// 头部: RSV(2) + FRAG(1) + ATYP(1) + DST.ADDR(N) + DST.PORT(2) + DATA

		// 使用目标服务器的地址作为 SOCKS5 响应头部的地址 (DST.ADDR)
		respHeader := bytes.NewBuffer([]byte{0x00, 0x00, 0x00, ATypIPv4}) // RSV, FRAG=0, ATYP=IPv4

		// 目标服务器 IP (假设是 IPv4)
		respHeader.Write(targetServerAddr.IP.To4())

		// 目标服务器端口
		port := uint16(targetServerAddr.Port)
		respHeader.WriteByte(byte(port >> 8))
		respHeader.WriteByte(byte(port & 0xff))

		// 2. 将头部和响应数据拼接
		finalPacket := append(respHeader.Bytes(), respBuf[:n]...)

		// 3. 发送给 SOCKS5 客户端
		if _, err := pc.WriteToUDP(finalPacket, clientAddr); err != nil {
			log.Printf("发送响应给客户端 %s 失败: %v", clientAddr.String(), err)
			return
		}
	}
}
