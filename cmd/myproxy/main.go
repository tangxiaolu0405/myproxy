package main

import (
	"fmt"
	"io"
	"log"

	"myproxy.com/p/internal/sockes5"
)

func main() {
	// 假设你的 SOCKS5 代理运行在本地 1080 端口
	client := &sockes5.SOCKS5Client{
		ProxyAddr: "127.0.0.1:1080",
		Username:  "", // 如果代理需要认证，在此填写
		Password:  "", // 如果代理需要认证，在此填写
	}

	// 想要连接的目标地址 (例如：Google 的 DNS 服务器)
	targetAddr := "www.baidu.com:80"

	// 使用 SOCKS5 客户端进行连接
	conn, err := client.Dial("tcp", targetAddr)
	if err != nil {
		log.Fatalf("通过代理连接目标服务器失败: %v", err)
	}
	defer conn.Close()

	log.Printf("成功通过 SOCKS5 代理连接到: %s", targetAddr)

	// 现在可以使用 conn 来发送和接收数据，就像使用普通的 net.Conn 一样

	// 示例：发送一个简单的 HTTP GET 请求
	request := "GET / HTTP/1.1\r\nHost: www.baidu.com\r\nConnection: close\r\n\r\n"
	if _, err := conn.Write([]byte(request)); err != nil {
		log.Fatalf("发送请求失败: %v", err)
	}

	// 示例：读取并打印响应
	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil && err != io.EOF {
		log.Printf("读取响应失败: %v", err)
	}

	fmt.Println("--- 目标服务器响应 (部分) ---")
	fmt.Println(string(response[:n]))
	fmt.Println("---------------------------")
}
