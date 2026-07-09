package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"myproxy.com/p/internal/model"
)

const trayPingRefreshInterval = 10 * time.Second

// TrayManager 管理系统托盘
type TrayManager struct {
	appState           *AppState
	app                fyne.App
	window             fyne.Window
	proxyModeMenuItems [2]*fyne.MenuItem // 系统代理模式菜单项（清除、系统）

	pingTicker  *time.Ticker
	pingStop    chan struct{}
	pingMu      sync.Mutex
	pingRunning bool

	serverMenuMu     sync.Mutex
	currentNodeID    string
	currentNodeLabel string
	proxyRunning     bool
	topAlternatives  []trayNodeEntry
	lastMenuSnapshot string
}

// NewTrayManager 创建系统托盘管理器
func NewTrayManager(appState *AppState) *TrayManager {
	return &TrayManager{
		appState: appState,
		app:      appState.App,
		window:   appState.Window,
	}
}

// getSystemProxyModeFromAppState 从 AppState（ConfigService）读取系统代理模式，与主窗口共用同一数据源。
func getSystemProxyModeFromAppState(a *AppState) SystemProxyMode {
	if a == nil || a.ConfigService == nil {
		return SystemProxyModeClear
	}
	s := a.ConfigService.GetSystemProxyMode()
	if s == "" {
		return SystemProxyModeClear
	}
	return ParseSystemProxyMode(s)
}

// SetupTray 设置系统托盘（使用 Fyne 原生系统托盘 API）
func (tm *TrayManager) SetupTray() {
	if desk, ok := tm.app.(desktop.App); ok {
		icon := createTrayIconResource(tm.appState)
		if icon == nil {
			tm.appState.SafeLogger.Warn("创建托盘图标失败")
			return
		}
		desk.SetSystemTrayIcon(icon)
		tm.syncCurrentNodeFromAppState(nil)
		tm.createTrayMenu(desk)
		tm.StartPingRefresh()
	} else {
		tm.appState.SafeLogger.Warn("应用不支持桌面扩展，无法显示系统托盘")
	}
}

// RefreshTrayIcon 根据当前主题刷新托盘图标（主题切换后调用）。
func (tm *TrayManager) RefreshTrayIcon() {
	if tm.appState == nil {
		return
	}
	if desk, ok := tm.app.(desktop.App); ok {
		icon := createTrayIconResource(tm.appState)
		if icon != nil {
			desk.SetSystemTrayIcon(icon)
		}
	}
}

// StartPingRefresh 启动托盘节点测速与 Top5 刷新（首次立即执行，之后每 10 秒）。
func (tm *TrayManager) StartPingRefresh() {
	tm.StopPingRefresh()
	stop := make(chan struct{})
	ticker := time.NewTicker(trayPingRefreshInterval)
	tm.pingStop = stop
	tm.pingTicker = ticker
	go func() {
		tm.runPingCycle()
		for {
			select {
			case <-ticker.C:
				tm.runPingCycle()
			case <-stop:
				return
			}
		}
	}()
}

// StopPingRefresh 停止托盘节点测速定时器。
func (tm *TrayManager) StopPingRefresh() {
	if tm.pingStop != nil {
		close(tm.pingStop)
		tm.pingStop = nil
	}
	if tm.pingTicker != nil {
		tm.pingTicker.Stop()
		tm.pingTicker = nil
	}
}

func (tm *TrayManager) runPingCycle() {
	tm.pingMu.Lock()
	if tm.pingRunning {
		tm.pingMu.Unlock()
		return
	}
	tm.pingRunning = true
	tm.pingMu.Unlock()

	defer func() {
		tm.pingMu.Lock()
		tm.pingRunning = false
		tm.pingMu.Unlock()
	}()

	if tm.appState == nil || tm.appState.Ping == nil || tm.appState.ServerService == nil {
		return
	}

	servers := tm.enabledServersFromList()
	if len(servers) == 0 {
		fyne.Do(func() {
			tm.applyPingResults(nil, nil)
		})
		return
	}

	results := tm.appState.Ping.TestAllServersDelay(servers)

	if tm.appState.Store != nil && tm.appState.Store.Nodes != nil {
		_ = tm.appState.Store.Nodes.UpdateDelays(results)
	}

	fyne.Do(func() {
		tm.applyPingResults(results, servers)
	})
}

func (tm *TrayManager) enabledServersFromList() []model.Node {
	if tm.appState == nil || tm.appState.ServerService == nil {
		return nil
	}
	all := tm.appState.ServerService.ListServers()
	enabled := make([]model.Node, 0, len(all))
	for _, s := range all {
		if s.Enabled {
			enabled = append(enabled, s)
		}
	}
	return enabled
}

func (tm *TrayManager) applyPingResults(delays map[string]int, servers []model.Node) {
	currentID := ""
	if tm.appState != nil && tm.appState.Store != nil && tm.appState.Store.Nodes != nil {
		currentID = tm.appState.Store.Nodes.GetSelectedID()
	}

	tm.syncCurrentNodeFromAppState(delays)

	if servers == nil {
		servers = tm.enabledServersFromList()
	}
	if delays == nil {
		delays = make(map[string]int, len(servers))
		for _, s := range servers {
			if s.Delay > 0 {
				delays[s.ID] = s.Delay
			}
		}
	}

	tm.serverMenuMu.Lock()
	tm.topAlternatives = pickTopAlternatives(servers, delays, currentID, trayTopAlternativesCount)
	tm.serverMenuMu.Unlock()

	tm.refreshTrayMenuIfNeeded()
}

func (tm *TrayManager) syncCurrentNodeFromAppState(delays map[string]int) {
	tm.serverMenuMu.Lock()
	defer tm.serverMenuMu.Unlock()

	tm.proxyRunning = tm.appState != nil &&
		tm.appState.XrayInstance != nil &&
		tm.appState.XrayInstance.IsRunning()

	var selected *model.Node
	if tm.appState != nil && tm.appState.Store != nil && tm.appState.Store.Nodes != nil {
		selected = tm.appState.Store.Nodes.GetSelected()
	}

	if selected == nil {
		tm.currentNodeID = ""
		tm.currentNodeLabel = "未选中节点"
		return
	}

	tm.currentNodeID = selected.ID
	delay := selected.Delay
	if delays != nil {
		if d, ok := delays[selected.ID]; ok && d > 0 {
			delay = d
		}
	}

	if tm.proxyRunning {
		if delay > 0 {
			tm.currentNodeLabel = fmt.Sprintf("%s (%dms)", selected.Name, delay)
		} else {
			tm.currentNodeLabel = selected.Name
		}
		return
	}
	tm.currentNodeLabel = fmt.Sprintf("当前选中: %s", selected.Name)
}

func (tm *TrayManager) menuSnapshotLocked() string {
	var b strings.Builder
	b.WriteString(tm.currentNodeID)
	b.WriteByte('|')
	b.WriteString(tm.currentNodeLabel)
	b.WriteByte('|')
	if tm.proxyRunning {
		b.WriteByte('1')
	} else {
		b.WriteByte('0')
	}
	for _, alt := range tm.topAlternatives {
		fmt.Fprintf(&b, "|%s:%d", alt.id, alt.delay)
	}
	mode := getSystemProxyModeFromAppState(tm.appState)
	b.WriteString("|mode:")
	b.WriteString(mode.ShortString())
	return b.String()
}

// RefreshTrayMenu 同步当前节点/代理状态并刷新托盘菜单（代理模式与节点区一并更新）。
func (tm *TrayManager) RefreshTrayMenu() {
	tm.syncCurrentNodeFromAppState(nil)
	tm.refreshTrayMenuIfNeeded()
}

func (tm *TrayManager) refreshTrayMenuIfNeeded() {
	tm.serverMenuMu.Lock()
	snapshot := tm.menuSnapshotLocked()
	changed := snapshot != tm.lastMenuSnapshot
	if changed {
		tm.lastMenuSnapshot = snapshot
	}
	tm.serverMenuMu.Unlock()

	if !changed {
		return
	}
	if desk, ok := tm.app.(desktop.App); ok {
		tm.createTrayMenu(desk)
	}
}

func (tm *TrayManager) buildServerMenuItems() []*fyne.MenuItem {
	tm.serverMenuMu.Lock()
	currentLabel := tm.currentNodeLabel
	currentID := tm.currentNodeID
	running := tm.proxyRunning
	alts := make([]trayNodeEntry, len(tm.topAlternatives))
	copy(alts, tm.topAlternatives)
	tm.serverMenuMu.Unlock()

	items := make([]*fyne.MenuItem, 0, len(alts)+3)

	currentItem := fyne.NewMenuItem(currentLabel, nil)
	if running && currentID != "" {
		currentItem.Checked = true
	}
	items = append(items, currentItem, fyne.NewMenuItemSeparator())

	if len(alts) == 0 {
		items = append(items, fyne.NewMenuItem("暂无可用节点", nil))
		return items
	}

	for _, alt := range alts {
		entry := alt
		label := fmt.Sprintf("%s (%dms)", entry.name, entry.delay)
		items = append(items, fyne.NewMenuItem(label, func() {
			tm.onAlternativeSelected(entry.id)
		}))
	}
	return items
}

func (tm *TrayManager) onAlternativeSelected(id string) {
	if tm.appState == nil || tm.appState.MainWindow == nil {
		return
	}
	if err := tm.appState.MainWindow.SwitchToServer(id); err != nil {
		if tm.appState.SafeLogger != nil {
			tm.appState.SafeLogger.Warn(fmt.Sprintf("托盘切换节点失败: %v", err))
		}
	}
}

// createTrayMenu 创建托盘菜单
func (tm *TrayManager) createTrayMenu(desk desktop.App) {
	// 创建系统代理模式菜单项（如果尚未创建）
	if tm.proxyModeMenuItems[0] == nil {
		tm.proxyModeMenuItems[0] = fyne.NewMenuItem(SystemProxyModeClear.ShortString(), func() {
			if tm.appState != nil && tm.appState.MainWindow != nil {
				_ = tm.appState.MainWindow.SetSystemProxyMode(SystemProxyModeClear)
			}
		})
		tm.proxyModeMenuItems[1] = fyne.NewMenuItem(SystemProxyModeAuto.ShortString(), func() {
			if tm.appState != nil && tm.appState.MainWindow != nil {
				_ = tm.appState.MainWindow.SetSystemProxyMode(SystemProxyModeAuto)
			}
		})
	}

	tm.updateProxyModeMenuCheckedState()

	closeProxyMenuItem := fyne.NewMenuItem("关闭代理", func() {
		if tm.appState != nil && tm.appState.MainWindow != nil {
			tm.appState.MainWindow.StopProxy()
			if tm.appState.MainWindow != nil {
				_ = tm.appState.MainWindow.SetSystemProxyMode(SystemProxyModeClear)
			}
		}
	})

	serverItems := tm.buildServerMenuItems()

	menuItems := []*fyne.MenuItem{
		fyne.NewMenuItem("显示窗口", func() {
			tm.window.Show()
			tm.window.RequestFocus()
		}),
		fyne.NewMenuItemSeparator(),
	}
	menuItems = append(menuItems, serverItems...)
	menuItems = append(menuItems,
		fyne.NewMenuItemSeparator(),
		closeProxyMenuItem,
		fyne.NewMenuItemSeparator(),
		tm.proxyModeMenuItems[0],
		tm.proxyModeMenuItems[1],
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("退出", func() {
			tm.quit()
		}),
	)

	menu := fyne.NewMenu("SOCKS5 代理客户端", menuItems...)
	desk.SetSystemTrayMenu(menu)

	tm.serverMenuMu.Lock()
	tm.lastMenuSnapshot = tm.menuSnapshotLocked()
	tm.serverMenuMu.Unlock()
}

// RefreshProxyModeMenu 刷新托盘菜单（代理模式与节点区）。
func (tm *TrayManager) RefreshProxyModeMenu() {
	tm.RefreshTrayMenu()
}

// updateProxyModeMenuCheckedState 从 AppState（ConfigService）读取系统代理模式，更新菜单选中状态。
func (tm *TrayManager) updateProxyModeMenuCheckedState() {
	if tm.appState == nil || tm.appState.ConfigService == nil {
		return
	}
	currentMode := getSystemProxyModeFromAppState(tm.appState)

	for i, item := range tm.proxyModeMenuItems {
		if item == nil {
			continue
		}
		switch i {
		case 0:
			item.Checked = (currentMode == SystemProxyModeClear)
		case 1:
			item.Checked = (currentMode == SystemProxyModeAuto)
		}
	}
}

// quit 退出应用
func (tm *TrayManager) quit() {
	tm.StopPingRefresh()

	if tm.appState.LogsPanel != nil {
		tm.appState.LogsPanel.Stop()
	}

	if tm.appState.MainWindow != nil {
		tm.appState.MainWindow.SaveLayoutConfig()
	}

	tm.appState.stopWindowSizeSaveTimer()
	if tm.appState.Window != nil && tm.appState.Window.Canvas() != nil {
		sz := tm.appState.Window.Canvas().Size()
		if sz.Width >= 200 && sz.Height >= 200 {
			tm.appState.SaveWindowSize(sz)
		}
	}

	tm.app.Quit()
}
