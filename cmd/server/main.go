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
	Version         = 0x05 // SOCKS5 版本
	AuthNoAuth      = 0x00 // 无认证
	CmdConnect      = 0x01 // CONNECT 命令
	CmdUdpAssociate = 0x03 // UDP ASSOCIATE 命令 (新增)
	ATypIPv4        = 0x01 // 地址类型：IPv4
	ATypDomain      = 0x03 // 地址类型：域名
	ReplySuccess    = 0x00 // 响应：成功
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

// 核心连接处理函数
func handleConnection(conn net.Conn) {
	defer conn.Close()

	// 1. 协商 (Negotiation)
	if err := doNegotiation(conn); err != nil {
		log.Printf("协商失败: %v", err)
		return
	}

	// 2. 读取请求头部: VER (1) + CMD (1) + RSV (1) + ATYP (1)
	requestHeader := make([]byte, 4)
	if _, err := io.ReadFull(conn, requestHeader); err != nil {
		log.Printf("读取请求头部失败: %v", err)
		return
	}

	// 验证 SOCKS5 版本
	if requestHeader[0] != Version {
		log.Printf("不支持的 SOCKS 版本: %d", requestHeader[0])
		return
	}

	cmd := requestHeader[1] // 获取命令类型

	if cmd == CmdConnect {
		// --- TCP 连接逻辑 ---
		if err := handleConnectRequest(conn, requestHeader); err != nil {
			log.Printf("CONNECT 请求处理失败: %v", err)
		}

	} else if cmd == CmdUdpAssociate {
		// --- UDP 关联逻辑 ---
		log.Println("收到 UDP ASSOCIATE 请求")
		// 在这里，我们将读取客户端的绑定地址，但由于我们不要求客户端绑定特定地址，
		// 所以只是简单读取并丢弃，然后调用处理函数。
		if _, err := readAddrPort(conn, requestHeader[3]); err != nil {
			log.Printf("读取 UDP ASSOCIATE 客户端地址失败: %v", err)
			return
		}

		if err := handleUDPAssociate(conn); err != nil {
			log.Printf("UDP ASSOCIATE 处理失败: %v", err)
		}

	} else {
		// 不支持的其他命令
		conn.Write([]byte{Version, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // 命令不支持
		log.Printf("不支持的 SOCKS 命令: %d", cmd)
	}
}

// 阶段 1: 协商 - 只支持无认证
func doNegotiation(conn net.Conn) error {
	// 客户端发送: VER (1) + NMETHODS (1) + METHODS (N)
	buf := make([]byte, 258)

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
	_, err := conn.Write([]byte{Version, AuthNoAuth})
	return err
}

// -----------------------------------------------------
// --- TCP CONNECT 逻辑 ---
// -----------------------------------------------------

// handleConnectRequest 处理 CONNECT 命令。
// requestHeader 包含 VER, CMD, RSV, ATYP 的前 4 字节。
func handleConnectRequest(conn net.Conn, requestHeader []byte) error {
	// 1. 读取目标地址和端口
	targetAddr, err := readAddrPort(conn, requestHeader[3])
	if err != nil {
		conn.Write([]byte{Version, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // 地址类型不支持
		return err
	}

	// 2. 尝试连接目标
	log.Printf("尝试连接目标: %s", targetAddr)
	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		conn.Write([]byte{Version, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // 连接目标失败
		return fmt.Errorf("连接目标 %s 失败: %w", targetAddr, err)
	}
	defer targetConn.Close()

	// 3. 响应成功
	successReply := []byte{Version, ReplySuccess, 0x00, ATypIPv4, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if _, err := conn.Write(successReply); err != nil {
		return fmt.Errorf("发送成功响应失败: %w", err)
	}

	// 4. 数据转发
	log.Printf("开始转发 TCP 数据...")
	// 使用 io.Copy 实现双向数据转发
	go io.Copy(targetConn, conn) // 客户端 -> 目标服务器
	io.Copy(conn, targetConn)    // 目标服务器 -> 客户端

	log.Printf("TCP 连接转发结束。")
	return nil
}

// 读取 SOCKS5 地址和端口
func readAddrPort(conn io.Reader, atyp byte) (string, error) {
	// 注意：这里将 conn 的类型改为 io.Reader，以适应 UDP 关联请求中对地址的读取
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

// -----------------------------------------------------
// --- UDP ASSOCIATE 逻辑 ---
// -----------------------------------------------------

// handleUDPAssociate 负责处理 UDP 关联请求并启动 UDP 转发
func handleUDPAssociate(conn net.Conn) error {
	// 1. 在代理服务器上创建一个临时的 UDP 监听端口
	udpAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	if err != nil {
		return err
	}
	pc, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("启动 UDP 监听失败: %w", err)
	}
	defer pc.Close()

	proxyUDPAddr := pc.LocalAddr().(*net.UDPAddr)
	log.Printf("为客户端分配 UDP 监听端口: %s", proxyUDPAddr.String())

	// 2. 发送成功响应给客户端 (通过 TCP)
	boundIP := net.ParseIP("127.0.0.1")

	resp := []byte{Version, ReplySuccess, 0x00, ATypIPv4}
	resp = append(resp, boundIP.To4()...)

	port := uint16(proxyUDPAddr.Port)
	resp = append(resp, byte(port>>8), byte(port&0xff))

	if _, err := conn.Write(resp); err != nil {
		return fmt.Errorf("发送 UDP 关联响应失败: %w", err)
	}

	// 3. 启动 UDP 转发循环
	// 我们用一个 map 来保存客户端的 UDP 地址，因为一个 SOCKS5 客户端可能从不同端口发送 UDP 包。
	// Key: clientAddr.String() -> Value: 用于转发的 map (targetAddr -> *net.UDPConn)
	clientRelayMap := make(map[string]map[string]*net.UDPConn)

	// 启动一个 goroutine 专门处理 UDP 数据
	go udpRelayLoop(pc, clientRelayMap)

	// 4. 阻塞 TCP 连接，保持 UDP 转发活跃
	log.Println("UDP 关联成功，等待 TCP 连接关闭以结束转发...")

	conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	buffer := make([]byte, 1)
	for {
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

	return nil
}

// udpRelayLoop 负责从 UDP 监听器接收数据，解析头部，转发给目标，并将目标响应发回
func udpRelayLoop(pc *net.UDPConn, clientRelayMap map[string]map[string]*net.UDPConn) {
	defer log.Println("UDP 转发循环退出。")

	buf := make([]byte, 65535)

	for {
		n, clientAddr, err := pc.ReadFromUDP(buf)
		if err != nil {
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

		// 2. 初始化或获取该客户端的转发映射表
		clientKey := clientAddr.String()
		if clientRelayMap[clientKey] == nil {
			clientRelayMap[clientKey] = make(map[string]*net.UDPConn)
		}
		relayMap := clientRelayMap[clientKey]

		// 3. 获取或创建到目标服务器的连接
		targetConn, ok := relayMap[targetAddr]
		if !ok {
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

		// 4. 转发数据到目标服务器
		if _, err := targetConn.Write(payload); err != nil {
			log.Printf("转发数据到目标 %s 失败: %v", targetAddr, err)
		}
	}
}

// parseSocks5UdpHeader 从 UDP 包中解析出目标地址和实际负载
func parseSocks5UdpHeader(reader *bytes.Reader, atyp byte) (string, []byte, error) {

	var addrLen int
	var addr string

	switch atyp {
	case ATypIPv4:
		addrLen = 4
		ipBytes := make([]byte, 4)
		if _, err := io.ReadFull(reader, ipBytes); err != nil {
			return "", nil, err
		}
		addr = net.IP(ipBytes).String()
	case ATypDomain:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(reader, lenBuf); err != nil {
			return "", nil, err
		}
		addrLen = int(lenBuf[0])
		domainBytes := make([]byte, addrLen)
		if _, err := io.ReadFull(reader, domainBytes); err != nil {
			return "", nil, err
		}
		addr = string(domainBytes)
	default:
		return "", nil, fmt.Errorf("不支持的地址类型: %d", atyp)
	}

	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, portBytes); err != nil {
		return "", nil, err
	}
	port := int(portBytes[0])<<8 | int(portBytes[1])

	// 实际的负载数据
	payload := make([]byte, reader.Len())
	if _, err := io.ReadFull(reader, payload); err != nil {
		return "", nil, err
	}

	return fmt.Sprintf("%s:%d", addr, port), payload, nil
}

// handleTargetResponse 监听目标服务器的响应，并封装发回给 SOCKS5 客户端
func handleTargetResponse(pc *net.UDPConn, clientAddr *net.UDPAddr, targetConn *net.UDPConn) {
	defer targetConn.Close()

	respBuf := make([]byte, 65535)

	for {
		n, targetServerAddr, err := targetConn.ReadFromUDP(respBuf)
		if err != nil {
			return
		}

		// 1. 封装 SOCKS5 UDP 响应头部
		respHeader := bytes.NewBuffer([]byte{0x00, 0x00, 0x00, ATypIPv4})

		// 使用目标服务器的 IP 作为 SOCKS5 响应头部的地址 (DST.ADDR)
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
