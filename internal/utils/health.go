package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
)

// ProbeThroughSOCKS5 经本地 SOCKS5 代理探测公网连通性（用于连接健康检查）。
func ProbeThroughSOCKS5(proxyHost string, proxyPort int, probeURL string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	if probeURL == "" {
		probeURL = "http://www.gstatic.com/generate_204"
	}
	socksAddr := fmt.Sprintf("%s:%d", proxyHost, proxyPort)
	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, &net.Dialer{Timeout: timeout})
	if err != nil {
		return fmt.Errorf("创建 SOCKS5 dialer 失败: %w", err)
	}
	contextDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return fmt.Errorf("SOCKS5 dialer 不支持 DialContext")
	}
	transport := &http.Transport{
		DialContext:           contextDialer.DialContext,
		ResponseHeaderTimeout: timeout,
		IdleConnTimeout:       timeout,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}
	return fmt.Errorf("探测返回状态码 %d", resp.StatusCode)
}
