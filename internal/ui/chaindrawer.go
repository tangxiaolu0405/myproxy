package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	// chainDrawerPanelMaxWidth 悬浮面板最大宽度（窗口更宽时不再加宽）。
	chainDrawerPanelMaxWidth = float32(460)
	// chainDrawerMargin 悬浮面板与窗口边缘的间距。
	chainDrawerMargin = float32(12)
)

// drawerDimColor 悬浮面板打开时覆盖主界面的遮罩颜色（半透明黑，深浅主题通用）。
var drawerDimColor = color.NRGBA{R: 0, G: 0, B: 0, A: 120}

// tapRect 可点击矩形（widget）：铺满覆盖层作为遮罩，点击关闭抽屉。
// 注意：必须实现 fyne.Widget（渲染器内绘制 canvas.Rectangle），
// 否则 Fyne 绘制器无法识别自定义 canvas 对象。
type tapRect struct {
	widget.BaseWidget
	rect  *canvas.Rectangle
	onTap func()
}

func newTapRect(fill color.Color, onTap func()) *tapRect {
	t := &tapRect{
		rect:  canvas.NewRectangle(fill),
		onTap: onTap,
	}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tapRect) Tapped(_ *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *tapRect) TappedSecondary(_ *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *tapRect) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.rect)
}

// absorbRect 吸收点击的矩形（widget）：作为面板背景层，
// 点击面板空白处不冒泡到遮罩层（不关闭抽屉）。
type absorbRect struct {
	widget.BaseWidget
	rect *canvas.Rectangle
}

func newAbsorbRect(fill, stroke color.Color) *absorbRect {
	r := canvas.NewRectangle(fill)
	r.CornerRadius = 8
	r.StrokeColor = stroke
	r.StrokeWidth = 1
	a := &absorbRect{rect: r}
	a.ExtendBaseWidget(a)
	return a
}

func (a *absorbRect) Tapped(_ *fyne.PointEvent)         {}
func (a *absorbRect) TappedSecondary(_ *fyne.PointEvent) {}

func (a *absorbRect) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(a.rect)
}

// drawerOverlayLayout 悬浮抽屉覆盖层布局：
//   - 遮罩（objs[0]）铺满整个覆盖层；
//   - 面板（objs[1]）右侧悬浮，四周留 chainDrawerMargin；
//   - 手柄（objs[2]）靠右边缘垂直居中。
type drawerOverlayLayout struct{}

func (drawerOverlayLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) < 3 {
		return
	}
	dim, panel, handle := objs[0], objs[1], objs[2]

	dim.Resize(size)
	dim.Move(fyne.NewPos(0, 0))

	panelW := fyne.Min(chainDrawerPanelMaxWidth, size.Width-2*chainDrawerMargin)
	panelH := size.Height - 2*chainDrawerMargin
	if panelW < 240 {
		panelW = 240
	}
	if panelH < 200 {
		panelH = 200
	}
	panel.Resize(fyne.NewSize(panelW, panelH))
	panel.Move(fyne.NewPos(size.Width-panelW-chainDrawerMargin, chainDrawerMargin))

	handleSize := handle.MinSize()
	handle.Resize(handleSize)
	handle.Move(fyne.NewPos(size.Width-handleSize.Width-4, (size.Height-handleSize.Height)/2))
}

func (drawerOverlayLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) >= 3 && objs[2] != nil {
		return objs[2].MinSize()
	}
	return fyne.NewSize(0, 0)
}

// ChainDrawer 主页右侧的链式代理悬浮抽屉：
// 主页右侧边缘一个 "<" 手柄按钮，点击后在主界面之上弹出悬浮面板（非全屏、不切换页面），
// 面板内复用 ChainPage 的链式配置（抽屉形态：标题栏右上关闭按钮替代返回按钮）。
// 遮罩覆盖主界面：点击面板外任意位置或面板右上角关闭按钮即收起。
type ChainDrawer struct {
	appState *AppState

	overlay   *fyne.Container // 覆盖层（遮罩 + 面板 + 手柄），BuildOverlay 构建
	dim       *tapRect        // 遮罩：点击关闭
	panel     *fyne.Container // 悬浮面板（卡片背景 + 链式配置内容）
	handle    *fyne.Container // 右侧 "<" 手柄标签
	handleBtn *widget.Button

	chain *ChainPage // 抽屉形态的链式配置页（复用实例，保持草稿）
	open  bool
}

// NewChainDrawer 创建主页右侧链式代理悬浮抽屉。
func NewChainDrawer(appState *AppState) *ChainDrawer {
	return &ChainDrawer{appState: appState}
}

// BuildOverlay 构建悬浮抽屉覆盖层（由主页 buildHomePage 调用，作为主页 Stack 的顶层）。
// 每次调用都会重建（主题切换等场景），旧的实例会被释放。
func (d *ChainDrawer) BuildOverlay() fyne.CanvasObject {
	d.dispose()

	// 遮罩：铺满主界面，点击关闭抽屉
	d.dim = newTapRect(drawerDimColor, d.Close)
	d.dim.Hide()

	// 悬浮面板：卡片背景（吸收空白处点击）+ 链式配置内容
	bg := newAbsorbRect(
		CurrentThemeColor(d.appState.App, theme.ColorNameInputBackground),
		CurrentThemeColor(d.appState.App, theme.ColorNameSeparator),
	)
	d.panel = container.NewStack(bg, d.buildChainContent())
	d.panel.Hide()

	// 右侧 "<" 手柄：圆角标签，点击开/关抽屉
	tabBg := canvas.NewRectangle(CurrentThemeColor(d.appState.App, theme.ColorNameInputBackground))
	tabBg.CornerRadius = 8
	tabBg.StrokeColor = CurrentThemeColor(d.appState.App, theme.ColorNameSeparator)
	tabBg.StrokeWidth = 1
	d.handleBtn = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), d.toggle)
	d.handleBtn.Importance = widget.LowImportance
	d.handle = container.NewStack(tabBg, newPaddedWithSize(d.handleBtn, 4))

	d.overlay = container.NewWithoutLayout(d.dim, d.panel, d.handle)
	d.overlay.Layout = drawerOverlayLayout{}
	return d.overlay
}

// buildChainContent 惰性创建抽屉形态的链式配置页（复用实例，草稿在开关抽屉间保留）。
func (d *ChainDrawer) buildChainContent() fyne.CanvasObject {
	if d.chain == nil {
		cp := NewChainPage(d.appState)
		cp.SetDrawerMode(d.Close)
		d.chain = cp
	}
	return d.chain.Build()
}

// toggle 手柄点击回调：开/关抽屉。
func (d *ChainDrawer) toggle() {
	if d.open {
		d.Close()
	} else {
		d.Open()
	}
}

// Open 打开悬浮抽屉：同步最新链草稿与节点列表，显示遮罩与右侧悬浮面板。
func (d *ChainDrawer) Open() {
	if d == nil || d.appState == nil || d.appState.Window == nil || d.open {
		return
	}
	if d.overlay == nil {
		d.BuildOverlay()
	}
	if d.chain != nil {
		d.chain.Refresh()
	}
	d.open = true
	if d.dim != nil {
		d.dim.Show()
	}
	if d.panel != nil {
		d.panel.Show()
	}
	if d.handle != nil {
		d.handle.Hide() // 打开时隐藏手柄，避免与面板重叠
	}
	if d.overlay != nil {
		d.overlay.Refresh()
	}
}

// Close 收起悬浮抽屉：隐藏遮罩与面板，恢复显示右侧手柄。
func (d *ChainDrawer) Close() {
	if d == nil {
		return
	}
	d.open = false
	if d.dim != nil {
		d.dim.Hide()
	}
	if d.panel != nil {
		d.panel.Hide()
	}
	if d.handle != nil {
		d.handle.Show()
	}
	if d.overlay != nil {
		d.overlay.Refresh()
	}
}

// IsOpen 返回抽屉当前是否打开。
func (d *ChainDrawer) IsOpen() bool {
	return d != nil && d.open
}

// dispose 释放当前覆盖层相关资源（主题重建或 Cleanup 时调用）。
func (d *ChainDrawer) dispose() {
	if d == nil {
		return
	}
	d.open = false
	if d.chain != nil {
		d.chain.Cleanup()
		d.chain = nil
	}
	d.overlay = nil
	d.dim = nil
	d.panel = nil
	d.handle = nil
	d.handleBtn = nil
}

// Cleanup 清理抽屉资源（窗口关闭时调用）。
func (d *ChainDrawer) Cleanup() {
	d.dispose()
}
