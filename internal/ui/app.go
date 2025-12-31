package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/theme"
	"myproxy.com/p/internal/logging"
	"myproxy.com/p/internal/ping"
	"myproxy.com/p/internal/server"
	"myproxy.com/p/internal/store"
	"myproxy.com/p/internal/subscription"
	"myproxy.com/p/internal/xray"
)

// AppState 管理应用的整体状态，包括管理器、日志和 UI 组件。
// 它作为应用的核心状态容器，协调各个组件之间的交互。
type AppState struct {
	PingManager *ping.PingManager
	Logger      *logging.Logger
	App         fyne.App
	Window      fyne.Window

	// Store - 数据层核心，管理所有数据和双向绑定
	Store *store.Store

	// Xray 实例 - 用于 xray-core 代理
	XrayInstance *xray.XrayInstance

	// 绑定数据 - 用于状态面板自动更新
	ProxyStatusBinding binding.String // 代理状态文本
	PortBinding        binding.String // 端口文本
	ServerNameBinding  binding.String // 服务器名称文本

	// 主窗口引用 - 用于刷新日志面板
	MainWindow *MainWindow

	// 日志面板引用 - 用于追加日志
	LogsPanel *LogsPanel

	// 内部 SubscriptionManager（仅用于订阅功能，不暴露为公共字段）
	subscriptionManager *subscription.SubscriptionManager
}

// NewAppState 创建并初始化新的应用状态。
// 参数：
//   - logger: 日志记录器
//
// 返回：初始化后的应用状态实例
func NewAppState(logger *logging.Logger) *AppState {
	// 创建 Store 实例
	dataStore := store.NewStore()

	// 创建绑定数据
	proxyStatusBinding := binding.NewString()
	portBinding := binding.NewString()
	serverNameBinding := binding.NewString()

	// 创建临时 ServerManager（用于 PingManager 和 SubscriptionManager）
	// TODO: 重构 PingManager 和 SubscriptionManager 使其直接使用 Store
	tempServerManager := server.NewServerManager(nil)
	pingManager := ping.NewPingManager(tempServerManager)
	subscriptionManager := subscription.NewSubscriptionManager(tempServerManager)

	appState := &AppState{
		PingManager:        pingManager,
		Logger:             logger,
		Store:              dataStore,
		ProxyStatusBinding: proxyStatusBinding,
		PortBinding:        portBinding,
		ServerNameBinding:  serverNameBinding,
		// 内部 SubscriptionManager（仅用于订阅功能，不暴露为字段）
		subscriptionManager: subscriptionManager,
	}

	// 注意：Store 数据加载将在 InitApp() 之后进行
	// 因为 Fyne 绑定需要在应用初始化后才能使用

	return appState
}

// updateStatusBindings 更新状态绑定数据
func (a *AppState) updateStatusBindings() {
	// 更新代理状态 - 基于实际运行的代理服务，而不是配置标志
	isRunning := false
	proxyPort := 0

	// 检查 xray 实例是否运行（使用 IsRunning 方法检查真实运行状态）
	if a.XrayInstance != nil && a.XrayInstance.IsRunning() {
		// xray-core 代理正在运行
		isRunning = true
		// 从 xray 实例获取端口
		if a.XrayInstance.GetPort() > 0 {
			proxyPort = a.XrayInstance.GetPort()
		} else {
			proxyPort = 10080 // 默认端口
		}
	}

	if isRunning {
		// 与 UI 设计规范保持一致的文案：当前连接状态 + 已连接
		a.ProxyStatusBinding.Set("当前连接状态: 🟢 已连接")
		if proxyPort > 0 {
			a.PortBinding.Set(fmt.Sprintf("监听端口: %d", proxyPort))
		} else {
			a.PortBinding.Set("监听端口: -")
		}
	} else {
		// 未连接状态文案
		a.ProxyStatusBinding.Set("当前连接状态: ⚪ 未连接")
		a.PortBinding.Set("监听端口: -")
	}

	// 更新当前服务器（符合 UI.md 设计：🌐 节点: US - LA - 32ms）
	if a.Store != nil && a.Store.Nodes != nil {
		selectedNode := a.Store.Nodes.GetSelected()
		if selectedNode != nil {
			// 使用节点名称，格式更简洁
			a.ServerNameBinding.Set(fmt.Sprintf("🌐 节点: %s", selectedNode.Name))
		} else {
			a.ServerNameBinding.Set("🌐 节点: 无")
		}
	} else {
		a.ServerNameBinding.Set("🌐 节点: 无")
	}
}

// UpdateProxyStatus 更新代理状态并刷新 UI 绑定数据。
// 该方法会检查代理转发器的实际运行状态，并更新相关的绑定数据，
// 使状态面板能够自动反映最新的代理状态。
func (a *AppState) UpdateProxyStatus() {
	a.updateStatusBindings()
}

// InitApp 初始化 Fyne 应用和窗口。
// 该方法会创建应用实例、设置主题、创建主窗口，并初始化数据绑定。
// 注意：必须在创建 UI 组件之前调用此方法。
func (a *AppState) InitApp() {
	a.App = app.NewWithID("com.myproxy.socks5")
	
	// 设置应用图标（使用自定义图标）
	// 这会同时设置 Dock 图标和窗口图标（在 macOS 上）
	appIcon := createAppIcon()
	if appIcon != nil {
		a.App.SetIcon(appIcon)
		fmt.Println("应用图标已设置（包括 Dock 图标）")
	} else {
		fmt.Println("警告: 应用图标创建失败")
	}
	
	// 从 Store 加载主题配置，默认使用黑色主题
	themeVariant := theme.VariantDark
	if a.Store != nil && a.Store.AppConfig != nil {
		if themeStr, err := a.Store.AppConfig.GetWithDefault("theme", "dark"); err == nil && themeStr == "light" {
			themeVariant = theme.VariantLight
		}
	}
	a.App.Settings().SetTheme(NewMonochromeTheme(themeVariant))
	a.Window = a.App.NewWindow("myproxy")
	// 从 Store 读取窗口大小，如果没有则使用默认值
	defaultSize := fyne.NewSize(420, 520)
	windowSize := LoadWindowSize(a, defaultSize)
	a.Window.Resize(windowSize)

	// Fyne 应用初始化后，可以初始化绑定数据
	// 先加载 Store 数据（必须在 Fyne 应用初始化后）
	if a.Store != nil {
		a.Store.LoadAll()
	}
	
	a.updateStatusBindings()

	// 注意：Logger的回调需要在LogsPanel创建后设置（在NewMainWindow之后）
}

// AppendLog 追加一条日志到日志面板（全局接口）
// 该方法可以从任何地方调用，会自动追加到日志缓冲区并更新显示
// 参数：
//   - level: 日志级别 (DEBUG, INFO, WARN, ERROR, FATAL)
//   - logType: 日志类型 (app 或 xray；其他将归并为 app)
//   - message: 日志消息
func (a *AppState) AppendLog(level, logType, message string) {
	// 规范化：级别大写，来源仅 app/xray
	level = strings.ToUpper(level)
	switch strings.ToLower(logType) {
	case "xray":
		logType = "xray"
	default:
		logType = "app"
	}
	if a.LogsPanel != nil {
		a.LogsPanel.AppendLog(level, logType, message)
	}
}

// LoadWindowSize 从 Store 加载窗口大小，如果不存在则返回默认值
// 参数：
//   - appState: 应用状态（包含 Store）
//   - defaultSize: 默认窗口大小
// 返回：窗口大小
func LoadWindowSize(appState *AppState, defaultSize fyne.Size) fyne.Size {
	if appState != nil && appState.Store != nil && appState.Store.AppConfig != nil {
		return appState.Store.AppConfig.GetWindowSize(defaultSize)
	}
	return defaultSize
}

// SaveWindowSize 保存窗口大小到 Store
// 参数：
//   - appState: 应用状态（包含 Store）
//   - size: 窗口大小
func SaveWindowSize(appState *AppState, size fyne.Size) {
	if appState != nil && appState.Store != nil && appState.Store.AppConfig != nil {
		_ = appState.Store.AppConfig.SaveWindowSize(size)
	}
}
