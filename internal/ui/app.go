package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/theme"
	"myproxy.com/p/internal/config"
	"myproxy.com/p/internal/database"
	"myproxy.com/p/internal/logging"
	"myproxy.com/p/internal/ping"
	"myproxy.com/p/internal/proxy"
	"myproxy.com/p/internal/server"
	"myproxy.com/p/internal/subscription"
)

// AppState 应用状态
type AppState struct {
	Config              *config.Config
	ServerManager       *server.ServerManager
	SubscriptionManager *subscription.SubscriptionManager
	PingManager         *ping.PingManager
	Logger              *logging.Logger
	App                 fyne.App
	Window              fyne.Window
	SelectedServerID    string

	// 代理转发器 - 用于实际启动和管理代理服务
	ProxyForwarder *proxy.Forwarder

	// 绑定数据 - 用于状态面板自动更新
	ProxyStatusBinding binding.String // 代理状态文本
	PortBinding        binding.String // 端口文本
	ServerNameBinding  binding.String // 服务器名称文本

	// 订阅标签绑定 - 用于订阅管理面板自动更新
	SubscriptionLabelsBinding binding.StringList // 订阅标签列表

	// 主窗口引用 - 用于刷新日志面板
	MainWindow *MainWindow
}

// NewAppState 创建新的应用状态
func NewAppState(cfg *config.Config, logger *logging.Logger) *AppState {
	serverManager := server.NewServerManager(cfg)
	subscriptionManager := subscription.NewSubscriptionManager(serverManager)
	pingManager := ping.NewPingManager(serverManager)

	// 创建绑定数据
	proxyStatusBinding := binding.NewString()
	portBinding := binding.NewString()
	serverNameBinding := binding.NewString()
	subscriptionLabelsBinding := binding.NewStringList()

	appState := &AppState{
		Config:                    cfg,
		ServerManager:             serverManager,
		SubscriptionManager:       subscriptionManager,
		PingManager:               pingManager,
		Logger:                    logger,
		SelectedServerID:          "",
		ProxyStatusBinding:        proxyStatusBinding,
		PortBinding:               portBinding,
		ServerNameBinding:         serverNameBinding,
		SubscriptionLabelsBinding: subscriptionLabelsBinding,
	}

	// 注意：不在构造函数中初始化绑定数据
	// 绑定数据需要在 Fyne 应用初始化后才能使用
	// 将在 InitApp() 之后初始化

	return appState
}

// updateStatusBindings 更新状态绑定数据
func (a *AppState) updateStatusBindings() {
	// 更新代理状态 - 基于实际运行的代理服务，而不是配置标志
	isRunning := false
	proxyPort := 0

	if a.ProxyForwarder != nil && a.ProxyForwarder.IsRunning {
		// 代理服务正在运行
		isRunning = true
		// 从本地地址中提取端口
		if a.Config != nil && a.Config.AutoProxyPort > 0 {
			proxyPort = a.Config.AutoProxyPort
		}
	}

	if isRunning {
		a.ProxyStatusBinding.Set("代理状态: 运行中")
		if proxyPort > 0 {
			a.PortBinding.Set(fmt.Sprintf("动态端口: %d", proxyPort))
		} else {
			a.PortBinding.Set("动态端口: -")
		}
	} else {
		a.ProxyStatusBinding.Set("代理状态: 未启动")
		a.PortBinding.Set("动态端口: -")
	}

	// 更新当前服务器
	if a.ServerManager != nil && a.SelectedServerID != "" {
		server, err := a.ServerManager.GetServer(a.SelectedServerID)
		if err == nil && server != nil {
			a.ServerNameBinding.Set(fmt.Sprintf("当前服务器: %s (%s:%d)", server.Name, server.Addr, server.Port))
		} else {
			a.ServerNameBinding.Set("当前服务器: 未知")
		}
	} else {
		a.ServerNameBinding.Set("当前服务器: 无")
	}
}

// UpdateProxyStatus 更新代理状态（供外部调用）
func (a *AppState) UpdateProxyStatus() {
	a.updateStatusBindings()
}

// InitApp 初始化应用
func (a *AppState) InitApp() {
	a.App = app.New()
	// 默认使用黑色主题（可改为 theme.VariantLight）
	a.App.Settings().SetTheme(NewMonochromeTheme(theme.VariantDark))
	a.Window = a.App.NewWindow("SOCKS5 代理客户端")
	a.Window.Resize(fyne.NewSize(900, 700))

	// Fyne 应用初始化后，可以初始化绑定数据
	a.updateStatusBindings()
	a.updateSubscriptionLabels()
}

// updateSubscriptionLabels 更新订阅标签绑定数据
func (a *AppState) updateSubscriptionLabels() {
	// 从数据库获取所有订阅
	subscriptions, err := database.GetAllSubscriptions()
	if err != nil {
		// 如果获取失败，设置为空列表
		a.SubscriptionLabelsBinding.Set([]string{})
		return
	}

	// 提取标签列表
	labels := make([]string, 0, len(subscriptions))
	for _, sub := range subscriptions {
		if sub.Label != "" {
			labels = append(labels, sub.Label)
		}
	}

	// 更新绑定数据
	a.SubscriptionLabelsBinding.Set(labels)
}

// UpdateSubscriptionLabels 更新订阅标签（供外部调用）
func (a *AppState) UpdateSubscriptionLabels() {
	a.updateSubscriptionLabels()
}
