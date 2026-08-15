package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ---------- 主题缩略图 ----------

// themeThumbPalette 主题缩略图使用的配色（直接取自 MonochromeTheme 的明/暗色板常量）。
type themeThumbPalette struct {
	background color.Color
	header     color.Color
	input      color.Color
	foreground color.Color
	primary    color.Color
	chart      color.Color
}

func darkThumbPalette() themeThumbPalette {
	return themeThumbPalette{
		background: hexToRGBA(DarkBackground),
		header:     hexToRGBA(DarkHeader),
		input:      hexToRGBA(DarkInputButton),
		foreground: hexToRGBA(DarkForeground),
		primary:    hexToRGBA(DarkPrimary),
		chart:      hexToRGBA(DarkChartSecondary),
	}
}

func lightThumbPalette() themeThumbPalette {
	return themeThumbPalette{
		background: hexToRGBA(LightBackground),
		header:     hexToRGBA(LightHeader),
		input:      hexToRGBA(LightInputButton),
		foreground: hexToRGBA(LightForeground),
		primary:    hexToRGBA(LightPrimary),
		chart:      hexToRGBA(LightChartSecondary),
	}
}

// miniWindowLayout 迷你“窗口”缩略图布局：元素按容器尺寸比例摆放，随卡片宽度缩放。
// 对象顺序：0 背景 / 1 顶栏 / 2 开关圆 / 3 节点行 / 4-6 底部图表条。
type miniWindowLayout struct {
	p themeThumbPalette
}

func (l miniWindowLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) < 7 || size.Width < 10 || size.Height < 10 {
		return
	}
	w, h := size.Width, size.Height

	objs[0].Resize(size)
	objs[0].Move(fyne.NewPos(0, 0))

	barH := h * 0.15
	objs[1].Resize(fyne.NewSize(w, barH))
	objs[1].Move(fyne.NewPos(0, 0))

	d := w * 0.32
	objs[2].Resize(fyne.NewSize(d, d))
	objs[2].Move(fyne.NewPos((w-d)/2, h*0.22))

	rowH := h * 0.14
	objs[3].Resize(fyne.NewSize(w*0.84, rowH))
	objs[3].Move(fyne.NewPos(w*0.08, h*0.52))

	barW := w * 0.14
	positions := []float32{w * 0.14, w * 0.43, w * 0.72}
	heights := []float32{h * 0.10, h * 0.16, h * 0.13}
	for i := 0; i < 3 && 4+i < len(objs); i++ {
		b := objs[4+i]
		bh := heights[i]
		b.Resize(fyne.NewSize(barW, bh))
		b.Move(fyne.NewPos(positions[i], h*0.86-bh))
	}
}

func (l miniWindowLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, 96)
}

// newMiniWindow 构建一个迷你“窗口”缩略图（背景 + 顶栏 + 主开关圆 + 节点行 + 流量条）。
func newMiniWindow(p themeThumbPalette) fyne.CanvasObject {
	bg := canvas.NewRectangle(p.background)
	bar := canvas.NewRectangle(p.header)
	toggle := canvas.NewCircle(p.primary)
	row := canvas.NewRectangle(p.input)
	row.CornerRadius = 2
	bars := make([]fyne.CanvasObject, 3)
	bars[0] = canvas.NewRectangle(p.chart)
	bars[1] = canvas.NewRectangle(p.primary)
	bars[2] = canvas.NewRectangle(p.chart)
	for _, b := range bars {
		b.(*canvas.Rectangle).CornerRadius = 1
	}
	objs := []fyne.CanvasObject{bg, bar, toggle, row, bars[0], bars[1], bars[2]}
	c := container.NewWithoutLayout(objs...)
	c.Layout = miniWindowLayout{p: p}
	return c
}

// splitThumbLayout 「跟随系统」缩略图：左深色、右浅色两个迷你窗口并排。
type splitThumbLayout struct{}

func (splitThumbLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) < 2 {
		return
	}
	half := size.Width / 2
	objs[0].Resize(fyne.NewSize(half, size.Height))
	objs[0].Move(fyne.NewPos(0, 0))
	objs[1].Resize(fyne.NewSize(size.Width-half, size.Height))
	objs[1].Move(fyne.NewPos(half, 0))
}

func (splitThumbLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, 96)
}

// buildThemeThumbnail 按主题显示名构建缩略图：深色/浅色为单窗口，跟随系统为左右双窗口。
func buildThemeThumbnail(display string) fyne.CanvasObject {
	switch display {
	case ThemeDisplayLight:
		return newMiniWindow(lightThumbPalette())
	case ThemeDisplaySystem:
		split := container.NewWithoutLayout(
			newMiniWindow(darkThumbPalette()),
			newMiniWindow(lightThumbPalette()),
		)
		split.Layout = splitThumbLayout{}
		return split
	default:
		return newMiniWindow(darkThumbPalette())
	}
}

// buildThemeCard 构建可点击的主题选择卡片：迷你窗口缩略图 + 主题名，选中态主色描边 + 加粗。
func (sp *SettingsPage) buildThemeCard(display string, thumb fyne.CanvasObject, selected bool) fyne.CanvasObject {
	pad := innerPadding(sp.appState)

	bg := newTapRect(CurrentThemeColor(sp.appState.App, theme.ColorNameInputBackground), func() { sp.onThemeChanged(display) })
	bg.rect.CornerRadius = 10

	border := canvas.NewRectangle(color.Transparent)
	border.CornerRadius = 10
	if selected {
		border.StrokeColor = CurrentThemeColor(sp.appState.App, theme.ColorNamePrimary)
		border.StrokeWidth = 2
	} else {
		border.StrokeColor = CurrentThemeColor(sp.appState.App, theme.ColorNameSeparator)
		border.StrokeWidth = 1
	}

	name := widget.NewLabel(display)
	name.Alignment = fyne.TextAlignCenter
	if selected {
		name.TextStyle = fyne.TextStyle{Bold: true}
	}

	content := container.NewVBox(thumb, name)
	return container.NewStack(bg, border, newPaddedWithSize(content, pad))
}

// ---------- 主界面实时预览 ----------

// previewChartLayout 预览流量图：竖向条按容器宽度均分。
type previewChartLayout struct{}

func (previewChartLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	n := len(objs)
	if n == 0 || size.Width < 10 {
		return
	}
	slot := size.Width / float32(n)
	for i, o := range objs {
		ratio := 0.35 + 0.4*float32(i%3)/2
		bh := size.Height * ratio
		bw := slot * 0.45
		o.Resize(fyne.NewSize(bw, bh))
		o.Move(fyne.NewPos(slot*float32(i)+(slot-bw)/2, size.Height-bh))
	}
}

func (previewChartLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(140, 44)
}

// buildMainInterfacePreview 构建当前主题下的主界面实时预览：
// 顶栏 + 主开关圆 + 节点行 + 模式按钮 + TUN 开关 + 流量图 + 端口，全部取当前主题色，
// 切换主题后设置页重建，预览随之实时更新。
func buildMainInterfacePreview(appState *AppState) fyne.CanvasObject {
	pad := innerPadding(appState)

	// 顶栏：logo + 订阅/设置
	var logo fyne.CanvasObject = widget.NewIcon(theme.ComputerIcon())
	if r := createHomeLogo(appState); r != nil {
		logo = widget.NewIcon(r)
	}
	header := container.NewHBox(
		logo,
		layout.NewSpacer(),
		widget.NewButtonWithIcon("订阅", theme.StorageIcon(), nil),
		widget.NewButtonWithIcon("设置", theme.SettingsIcon(), nil),
	)

	// 主开关圆（状态跟随实际代理运行状态）
	var toggle *CircularButton
	const toggleSize = 64
	if appState != nil && appState.XrayInstance != nil && appState.XrayInstance.IsRunning() {
		toggle = NewCircularButton(theme.CancelIcon(), nil, toggleSize, appState)
		toggle.SetActive(true)
	} else {
		toggle = NewCircularButton(theme.ConfirmIcon(), nil, toggleSize, appState)
	}
	toggleArea := container.NewCenter(toggle)

	// 节点行：显示当前节点/链名称
	displayName := "无"
	if appState != nil && appState.MainWindow != nil {
		if n := appState.MainWindow.proxyDisplayName(); n != "" {
			displayName = n
		}
	} else if appState != nil && appState.Store != nil && appState.Store.Nodes != nil {
		if n := appState.Store.Nodes.GetSelected(); n != nil {
			displayName = n.Name
		}
	}
	nodeBtn := widget.NewButton(truncateDisplayText(displayName, 22), nil)
	nodeBtn.Importance = widget.LowImportance

	// 模式按钮：清除 / 系统（选中态跟随当前配置）
	clearBtn := widget.NewButtonWithIcon("清除", theme.DeleteIcon(), nil)
	clearBtn.Importance = widget.LowImportance
	sysBtn := widget.NewButtonWithIcon("系统", theme.ComputerIcon(), nil)
	sysBtn.Importance = widget.LowImportance
	if appState != nil && appState.ConfigService != nil &&
		ParseSystemProxyMode(appState.ConfigService.GetSystemProxyMode()) == SystemProxyModeAuto {
		sysBtn.Importance = widget.HighImportance
	} else {
		clearBtn.Importance = widget.HighImportance
	}
	modeRow := container.NewGridWithColumns(2, clearBtn, sysBtn)

	// TUN 开关
	tunCheck := widget.NewCheck("TUN 全局（游戏/任意端口）", nil)
	if appState != nil && appState.ConfigService != nil {
		tunCheck.SetChecked(appState.ConfigService.IsTunMode())
	}

	// 流量图：几根竖向条
	chart := container.NewWithoutLayout()
	chart.Layout = previewChartLayout{}
	primary := CurrentThemeColor(appState.App, theme.ColorNamePrimary)
	secondary := ChartDownloadColor(appState.App)
	for i := 0; i < 5; i++ {
		b := canvas.NewRectangle(primary)
		if i%2 == 1 {
			b.FillColor = secondary
		}
		b.CornerRadius = 2
		chart.Add(b)
	}

	// 底栏：端口
	port := 0
	if appState != nil {
		port = appState.EffectiveLocalInboundPort()
	}
	footer := widget.NewLabel(fmt.Sprintf("端口 %d", port))
	footer.Alignment = fyne.TextAlignCenter
	footer.Importance = widget.LowImportance

	cardBg := canvas.NewRectangle(CurrentThemeColor(appState.App, theme.ColorNameInputBackground))
	cardBg.CornerRadius = 10
	inner := container.NewVBox(
		header,
		widget.NewSeparator(),
		toggleArea,
		nodeBtn,
		modeRow,
		tunCheck,
		chart,
		footer,
	)
	return container.NewStack(cardBg, newPaddedWithSize(inner, pad))
}
