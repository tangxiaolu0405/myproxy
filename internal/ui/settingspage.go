package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// SettingsPage 管理应用设置的显示和操作
// 它支持主题设置、代理设置、日志设置等功能
// 参考 NodePage 和 SubscriptionPage 的实现模式
type SettingsPage struct {
	appState *AppState
	content  fyne.CanvasObject // 内容容器
}

// NewSettingsPage 创建设置页面实例
func NewSettingsPage(appState *AppState) *SettingsPage {
	sp := &SettingsPage{
		appState: appState,
	}

	return sp
}

// Build 构建设置页面 UI
func (sp *SettingsPage) Build() fyne.CanvasObject {
	// 1. 返回按钮
	backBtn := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		if sp.appState != nil && sp.appState.MainWindow != nil {
			sp.appState.MainWindow.Back()
		}
	})
	backBtn.Importance = widget.LowImportance

	// 2. 标题
	titleLabel := NewTitleLabel("设置")

	// 3. 头部栏
	headerBar := container.NewPadded(container.NewHBox(
		backBtn,
		NewSpacer(SpacingLarge),
		titleLabel,
		layout.NewSpacer(),
	))

	// 4. 外观设置
	appearanceSection := container.NewVBox(
		widget.NewLabelWithStyle("外观", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		container.NewPadded(widget.NewButtonWithIcon("主题设置", theme.ColorPaletteIcon(), sp.handleThemeSettings)),
	)

	// 5. 代理设置
	proxySection := container.NewVBox(
		widget.NewLabelWithStyle("代理", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		container.NewPadded(widget.NewButtonWithIcon("自动启动代理", theme.MediaPlayIcon(), sp.handleAutoStartSettings)),
	)

	// 6. 日志设置
	logSection := container.NewVBox(
		widget.NewLabelWithStyle("日志", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		container.NewPadded(widget.NewButtonWithIcon("日志级别", theme.ViewRefreshIcon(), sp.handleLogLevelSettings)),
	)

	// 7. 关于设置
	aboutSection := container.NewVBox(
		widget.NewLabelWithStyle("关于", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		container.NewPadded(widget.NewButtonWithIcon("关于应用", theme.InfoIcon(), sp.handleAboutSettings)),
	)

	// 8. 组合所有设置项
	settingsList := container.NewVBox(
		appearanceSection,
		proxySection,
		logSection,
		aboutSection,
	)

	// 9. 组合头部和内容
	sp.content = container.NewBorder(
		headerBar,
		nil, nil, nil,
		container.NewPadded(settingsList),
	)

	return sp.content
}

// handleThemeSettings 处理主题设置
func (sp *SettingsPage) handleThemeSettings() {
	if sp.appState == nil || sp.appState.App == nil {
		return
	}

	// 获取当前主题
	currentTheme := "dark"
	if sp.appState.Store != nil && sp.appState.Store.AppConfig != nil {
		if themeStr, err := sp.appState.Store.AppConfig.GetWithDefault("theme", "dark"); err == nil {
			currentTheme = themeStr
		}
	}

	// 切换主题
	newTheme := "light"
	if currentTheme == "light" {
		newTheme = "dark"
	}

	// 保存主题设置
	if sp.appState.Store != nil && sp.appState.Store.AppConfig != nil {
		_ = sp.appState.Store.AppConfig.Set("theme", newTheme)
	}

	// 应用新主题
	themeVariant := theme.VariantDark
	if newTheme == "light" {
		themeVariant = theme.VariantLight
	}
	sp.appState.App.Settings().SetTheme(NewMonochromeTheme(themeVariant))
}

// handleAutoStartSettings 处理自动启动设置
func (sp *SettingsPage) handleAutoStartSettings() {
	if sp.appState == nil || sp.appState.Store == nil || sp.appState.Store.AppConfig == nil {
		return
	}

	// 获取当前设置
	currentAutoStart, _ := sp.appState.Store.AppConfig.GetWithDefault("autoStartProxy", "false")
	newAutoStart := "true"
	if currentAutoStart == "true" {
		newAutoStart = "false"
	}

	// 保存设置
	_ = sp.appState.Store.AppConfig.Set("autoStartProxy", newAutoStart)

	// 显示提示
	status := "已启用"
	if newAutoStart == "false" {
		status = "已禁用"
	}
	if sp.appState.Window != nil {
		dialog.ShowInformation("设置成功", "自动启动代理: "+status, sp.appState.Window)
	}
}

// handleLogLevelSettings 处理日志级别设置
func (sp *SettingsPage) handleLogLevelSettings() {
	if sp.appState == nil || sp.appState.Logger == nil {
		return
	}

	// 获取当前日志级别
	currentLevel := sp.appState.Logger.GetLogLevel()
	newLevel := "info"
	if currentLevel == "info" {
		newLevel = "debug"
	}

	// 设置新日志级别
	sp.appState.Logger.SetLogLevel(newLevel)

	// 保存设置
	if sp.appState.Store != nil && sp.appState.Store.AppConfig != nil {
		_ = sp.appState.Store.AppConfig.Set("logLevel", newLevel)
	}

	// 显示提示
	if sp.appState.Window != nil {
		dialog.ShowInformation("设置成功", "日志级别: "+newLevel, sp.appState.Window)
	}
}

// handleAboutSettings 处理关于应用设置
func (sp *SettingsPage) handleAboutSettings() {
	if sp.appState == nil || sp.appState.Window == nil {
		return
	}

	dialog.ShowInformation(
		"关于 myproxy",
		"版本: 1.0.0\n\n一个轻量级的代理管理工具\n基于 Xray-core 和 Fyne 框架",
		sp.appState.Window,
	)
}
