package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"myproxy.com/p/internal/database"
	"myproxy.com/p/internal/logging"
	"myproxy.com/p/internal/ui"
)

func main() {
	// 获取工作目录（用于确定数据库路径）
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("获取工作目录失败: %v", err)
	}

	// 初始化数据库（使用硬编码路径，避免暴露配置）
	dbPath := filepath.Join(workDir, "data", "myproxy.db")
	if err := database.InitDB(dbPath); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer database.CloseDB()

	// 初始化默认配置（如果不存在则写入硬编码默认值）
	if err := database.InitDefaultConfig(); err != nil {
		log.Printf("初始化默认配置失败: %v", err)
		// 继续执行，不影响应用启动
	}

	// 创建应用状态（先创建，logger稍后设置）
	appState := ui.NewAppState(nil)

	// 初始化应用（创建Fyne应用和窗口，并加载 Store 数据）
	appState.InitApp()

	// 创建主窗口（此时LogsPanel已创建）
	mainWindow := ui.NewMainWindow(appState)

	// 创建日志回调函数，用于实时更新UI（确保日志文件写入和UI显示一致）
	logCallback := func(level, logType, message, logLine string) {
		if appState.LogsPanel != nil {
			// 直接使用完整的日志行，确保格式与文件中的格式完全一致
			appState.LogsPanel.AppendLogLine(logLine)
		}
	}

	// 从 Store 读取日志配置并初始化 logger（使用硬编码默认值）
	logFile := "myproxy.log"
	logLevel := "info"
	if appState.Store != nil && appState.Store.AppConfig != nil {
		if file, err := appState.Store.AppConfig.GetWithDefault("logFile", "myproxy.log"); err == nil {
			logFile = file
		}
		if level, err := appState.Store.AppConfig.GetWithDefault("logLevel", "info"); err == nil {
			logLevel = level
		}
	}

	logger, err := logging.NewLogger(logFile, logLevel == "debug", logLevel, logCallback)
	if err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}

	// 设置logger到appState
	appState.Logger = logger

	// Logger初始化后，启动日志文件监控（用于监控xray日志等直接从文件写入的日志）
	if appState.LogsPanel != nil {
		appState.LogsPanel.StartLogFileWatcher()
	}

	// 设置窗口内容
	content := mainWindow.Build()
	if content != nil {
		appState.Window.SetContent(content)
	}

	// 创建并设置系统托盘（使用 Fyne 原生 API，不需要单独的 goroutine）
	trayManager := ui.NewTrayManager(appState)
	fmt.Println("开始设置系统托盘...")
	trayManager.SetupTray()
	fmt.Println("系统托盘设置完成")

	// 设置窗口关闭事件，隐藏到托盘而不是退出
	appState.Window.SetCloseIntercept(func() {
		// 保存窗口大小到数据库（通过 Store）
		if appState.Window != nil && appState.Window.Canvas() != nil {
			ui.SaveWindowSize(appState, appState.Window.Canvas().Size())
		}
		// 保存布局配置到数据库（通过 Store）
		mainWindow.SaveLayoutConfig()
		// 配置已由 Store 自动管理，无需手动保存
		// 隐藏窗口而不是关闭（Fyne 会自动处理 Dock 图标点击显示窗口）
		appState.Window.Hide()
	})
	fmt.Println("设置窗口关闭事件")

	// 显示窗口并运行应用
	appState.Window.Show()
	appState.App.Run()
	fmt.Println("应用运行结束")
}
