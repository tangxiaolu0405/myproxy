package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"time"

	"myproxy.com/p/internal/sockes5"
)

// -----------------------------------------------------
// --- 演示测试用例 ---
// -----------------------------------------------------

// testTCP 演示 SOCKS5 TCP CONNECT 功能 (用于 HTTP GET 请求)
func testTCP(client *sockes5.SOCKS5Client, targetAddr string) {
	log.Printf("\n--- [测试 1: TCP CONNECT] ---")
	log.Printf("目标: %s", targetAddr)

	conn, err := client.Dial("TCP", targetAddr)
	if err != nil {
		log.Fatalf("TCP CONNECT 失败: %v", err)
	}
	defer conn.Close()

	log.Printf("成功通过 SOCKS5 代理连接到: %s", targetAddr)

	// 示例：发送一个简单的 HTTP GET 请求
	host, _, _ := net.SplitHostPort(targetAddr)
	request := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", host)

	if _, err := conn.Write([]byte(request)); err != nil {
		log.Fatalf("发送 HTTP 请求失败: %v", err)
	}

	// 示例：读取并打印响应
	response := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(response)

	if err != nil && err != io.EOF {
		log.Printf("读取响应失败: %v", err)
	}

	fmt.Println("--- 目标服务器响应 (HTTP 头部) ---")
	fmt.Println(string(response[:n]))
	fmt.Println("---------------------------------")
}

// BuildUDPPacket 封装 SOCKS5 UDP 数据报头部
// targetAddr 格式如 "8.8.8.8:53"
func BuildUDPPacket(targetAddr string, data []byte) ([]byte, error) {
	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return nil, fmt.Errorf("目标地址格式错误: %w", err)
	}
	port, _ := strconv.Atoi(portStr)

	// 头部: RSV (2) + FRAG (1) + ATYP (1)
	// FRAG=0 表示这是一个完整的包
	buf := bytes.NewBuffer([]byte{0x00, 0x00, 0x00})

	// 目标地址类型和值
	buf.WriteByte(sockes5.ATypDomain)
	buf.WriteByte(byte(len(host)))
	buf.WriteString(host)

	// 端口 (2 bytes 大端序)
	buf.WriteByte(byte(port >> 8))
	buf.WriteByte(byte(port & 0xff))

	// 负载数据
	buf.Write(data)

	return buf.Bytes(), nil
}

// testUDP 演示 SOCKS5 UDP ASSOCIATE 功能 (用于 DNS 查询)
func testUDP(client *sockes5.SOCKS5Client, targetDNSAddr string) {
	log.Printf("\n--- [测试 2: UDP ASSOCIATE] ---")
	log.Printf("目标 DNS 服务器: %s", targetDNSAddr)

	// 1. 发起 UDP 关联 (通过 TCP)
	tcpConn, proxyUDPAddr, err := client.UDPAssociate()
	if err != nil {
		log.Fatalf("UDP ASSOCIATE 失败: %v", err)
	}
	// 保持 TCP 连接活跃，直到 UDP 测试结束
	defer tcpConn.Close()

	log.Printf("成功获取代理服务器 UDP 地址: %s:%d", proxyUDPAddr.Host, proxyUDPAddr.Port)

	// 2. 建立本地 UDP 连接到代理的 UDP 端口
	proxyNetAddr := fmt.Sprintf("%s:%d", proxyUDPAddr.Host, proxyUDPAddr.Port)
	localUDPConn, err := net.Dial("udp", proxyNetAddr)
	if err != nil {
		log.Fatalf("连接代理 UDP 端口失败: %v", err)
	}
	defer localUDPConn.Close()

	// 3. 构造 DNS 查询包 (查询 "example.com" 的 A 记录)
	// 这是一个标准的 DNS 头部和查询记录的硬编码示例
	dnsQuery := []byte{
		0x12, 0x34, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x07, 0x65, 0x78, 0x61,
		0x6d, 0x70, 0x6c, 0x65, 0x03, 0x63, 0x6f, 0x6d,
		0x00, 0x00, 0x01, 0x00, 0x01,
	}

	// 4. 封装 SOCKS5 UDP 数据报
	socks5Packet, err := BuildUDPPacket(targetDNSAddr, dnsQuery)
	if err != nil {
		log.Fatalf("封装 SOCKS5 UDP 数据包失败: %v", err)
	}

	log.Printf("发送 SOCKS5/UDP DNS 查询包到代理...")
	if _, err := localUDPConn.Write(socks5Packet); err != nil {
		log.Fatalf("发送 UDP 数据失败: %v", err)
	}

	// 5. 接收响应
	responseBuf := make([]byte, 1024)
	localUDPConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := localUDPConn.Read(responseBuf)

	if err != nil {
		log.Printf("接收 UDP 响应失败 (可能超时或代理不支持 UDP 转发): %v", err)
		return
	}

	// 6. 简单解析 SOCKS5 UDP 响应
	// 响应头部最小 10 字节 (RSV+FRAG+ATYP+ADDR+PORT)
	if n < 10 {
		log.Fatalf("接收到的数据太短: %d 字节", n)
	}

	// 简单检查 RSV/FRAG
	if responseBuf[0] != 0x00 || responseBuf[1] != 0x00 || responseBuf[2] != 0x00 {
		log.Println("警告: SOCKS5 UDP 响应头部 (RSV/FRAG) 校验失败")
	}

	// 简单地跳过前 10 字节头部 (假设 ATYP 是 IPv4)
	// 实际应根据 ATYP 动态计算长度
	dnsResponse := responseBuf[10:n]

	log.Printf("成功接收到 %d 字节的 DNS 响应数据。", len(dnsResponse))

	// 检查响应ID是否匹配
	if len(dnsResponse) >= 2 && dnsResponse[0] == dnsQuery[0] && dnsResponse[1] == dnsQuery[1] {
		log.Println("结果: DNS 响应事务ID匹配。UDP  ASSOCIATE 测试成功！")
	} else {
		log.Println("结果: DNS 响应数据校验未通过，原始数据：", dnsResponse)
	}
}

func main() {
	// 假设你的 SOCKS5 代理运行在本地 1080 端口
	client := &sockes5.SOCKS5Client{
		ProxyAddr: "127.0.0.1:1080",
		Username:  "", // 如果代理需要认证，在此填写
		Password:  "", // 如果代理需要认证，在此填写
	}

	// --- 运行 TCP CONNECT 测试 ---
	testTCP(client, "www.baidu.com:80")

	// --- 运行 UDP ASSOCIATE 测试 ---
	// 使用公共 DNS 服务器地址
	testUDP(client, "8.8.8.8:53")
}
