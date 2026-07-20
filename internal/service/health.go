package service

import (
	"fmt"
	"sync"
	"time"

	"myproxy.com/p/internal/database"
	"myproxy.com/p/internal/utils"
)

const (
	defaultHealthInterval = 60 * time.Second
	defaultHealthTimeout  = 8 * time.Second
	defaultHealthProbeURL = "http://www.gstatic.com/generate_204"
	healthFailThreshold   = 2
)

// HealthAlertCallback 连接健康告警回调（在探测 goroutine 中调用）。
type HealthAlertCallback func(message string)

// HealthCheckService 在代理运行时定期经本地入站探测公网连通性。
type HealthCheckService struct {
	mu            sync.Mutex
	running       bool
	stopCh        chan struct{}
	interval      time.Duration
	failCount     int
	alerted       bool
	getPort       func() int
	isProxyUp     func() bool
	onAlert       HealthAlertCallback
	probeURL      string
}

// NewHealthCheckService 创建健康检查服务。
func NewHealthCheckService(isProxyUp func() bool, getPort func() int, onAlert HealthAlertCallback) *HealthCheckService {
	return &HealthCheckService{
		interval:  defaultHealthInterval,
		getPort:   getPort,
		isProxyUp: isProxyUp,
		onAlert:   onAlert,
		probeURL:  defaultHealthProbeURL,
	}
}

// Start 启动定期探测（可重复调用，已运行则忽略）。
func (h *HealthCheckService) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running {
		return
	}
	h.running = true
	h.failCount = 0
	h.alerted = false
	h.stopCh = make(chan struct{})
	go h.loop(h.stopCh)
}

// Stop 停止探测。
func (h *HealthCheckService) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.running {
		return
	}
	close(h.stopCh)
	h.running = false
	h.failCount = 0
	h.alerted = false
}

// NotifyProxyStarted 代理启动后调用：启动探测并重置失败计数。
func (h *HealthCheckService) NotifyProxyStarted() {
	h.mu.Lock()
	h.failCount = 0
	h.alerted = false
	h.mu.Unlock()
	h.Start()
}

// NotifyProxyStopped 代理停止后调用。
func (h *HealthCheckService) NotifyProxyStopped() {
	h.Stop()
}

// SyncWithProxyState 按当前代理运行状态同步探测（仅在状态变化时启停，避免重复重置失败计数）。
func (h *HealthCheckService) SyncWithProxyState(proxyRunning bool) {
	h.mu.Lock()
	already := h.running
	h.mu.Unlock()
	if proxyRunning {
		if !already {
			h.NotifyProxyStarted()
		}
		return
	}
	if already {
		h.NotifyProxyStopped()
	}
}

func (h *HealthCheckService) loop(stop <-chan struct{}) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()
	// 启动后稍等再测，避免刚启动尚未就绪
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-stop:
		return
	case <-timer.C:
		h.checkOnce()
	}
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			h.checkOnce()
		}
	}
}

func (h *HealthCheckService) checkOnce() {
	if h.isProxyUp == nil || !h.isProxyUp() {
		return
	}
	port := database.DefaultMixedInboundPort
	if h.getPort != nil {
		if p := h.getPort(); p > 0 {
			port = p
		}
	}
	err := utils.ProbeThroughSOCKS5(database.LocalMixedInboundListenHost, port, h.probeURL, defaultHealthTimeout)
	h.mu.Lock()
	defer h.mu.Unlock()
	if err == nil {
		h.failCount = 0
		h.alerted = false
		return
	}
	h.failCount++
	if h.failCount >= healthFailThreshold && !h.alerted {
		h.alerted = true
		msg := fmt.Sprintf("连接健康检查失败：经本地代理无法访问外网（连续 %d 次）。请检查节点或网络。详情: %v", h.failCount, err)
		if h.onAlert != nil {
			h.onAlert(msg)
		}
	}
}
