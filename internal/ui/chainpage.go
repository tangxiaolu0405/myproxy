package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"myproxy.com/p/internal/model"
	"myproxy.com/p/internal/service"
)

// chainRowHeight 链内节点行高（与 ChainTargetItem.MinSize 一致，用于计算插入位置）。
const chainRowHeight = 44

// ChainPage 链式代理配置页。
// 左侧为链（可拖拽换序、移除），右侧为节点搜索列表（拖拽节点生成悬浮矩形框，按上下位置并入左侧链）。
// 草稿仅存内存，点击「保存」后持久化到 Store.Chain 并切换为链式代理模式；未保存不生效。
type ChainPage struct {
	appState *AppState
	content  fyne.CanvasObject

	// 右侧节点搜索与列表
	searchEntry    *widget.Entry
	searchText     string
	favoritesOnly  bool
	favoritesCheck *widget.Check
	nodeList       *widget.List
	nodeScroll     *container.Scroll
	listener       binding.DataListener

	// 左侧链草稿（仅内存；保存时写入 ChainStore）
	draftIDs      []string
	chainBox      *fyne.Container // 左侧链行容器（VBox）
	chainRows     []*ChainTargetItem
	chainListArea *container.Scroll // 左侧滚动区（用于计算插入位置）
	dropLine      *canvas.Rectangle // 插入指示线（绝对定位在 chainBox 之上）
	countLabel    *widget.Label
	emptyHint     *widget.Label

	// 拖拽状态
	dragNodeID       string
	dragNodeName     string
	reorderSourceIdx int // 链内换序时的源下标（-1 表示来自右侧列表）
	lastDragPos      fyne.Position
	dragPopUp        *widget.PopUp
	dragNameLabel    *widget.Label
	dropIndex        int // -1 表示指针不在左侧区域
}

// NewChainPage 创建链式代理配置页实例，草稿从 Store 当前链初始化。
func NewChainPage(appState *AppState) *ChainPage {
	cp := &ChainPage{
		appState:         appState,
		dropIndex:        -1,
		reorderSourceIdx: -1,
		draftIDs:         make([]string, 0),
	}
	if appState != nil && appState.Store != nil && appState.Store.Chain != nil {
		cp.draftIDs = appState.Store.Chain.GetNodeIDs()
	}
	// 监听节点数据变化，自动刷新右侧列表
	if appState != nil && appState.Store != nil && appState.Store.Nodes != nil {
		cp.listener = binding.NewDataListener(func() {
			if cp.nodeList != nil {
				cp.nodeList.Refresh()
			}
		})
		appState.Store.Nodes.NodesBinding.AddListener(cp.listener)
	}
	return cp
}

// Cleanup 释放页面持有的监听器。
func (cp *ChainPage) Cleanup() {
	if cp == nil || cp.listener == nil || cp.appState == nil || cp.appState.Store == nil || cp.appState.Store.Nodes == nil {
		return
	}
	cp.appState.Store.Nodes.NodesBinding.RemoveListener(cp.listener)
	cp.listener = nil
	cp.hideDragPopUp()
}

// Build 构建链式代理配置页 UI。
func (cp *ChainPage) Build() fyne.CanvasObject {
	pad := innerPadding(cp.appState)

	// 左上角返回按钮（与其他页面一致）
	backBtn := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		if cp.appState != nil && cp.appState.MainWindow != nil {
			cp.appState.MainWindow.Back()
		}
	})
	backBtn.Importance = widget.LowImportance

	title := NewTitleLabel("链式代理")

	// 右上角：清空 + 保存
	clearBtn := widget.NewButtonWithIcon("清空", theme.DeleteIcon(), cp.onClear)
	clearBtn.Importance = widget.LowImportance
	saveBtn := widget.NewButtonWithIcon("保存", theme.DocumentSaveIcon(), cp.onSave)
	saveBtn.Importance = widget.HighImportance

	headerBar := container.NewBorder(
		nil, nil,
		backBtn,
		container.NewHBox(clearBtn, saveBtn),
		container.NewHBox(title, layout.NewSpacer()),
	)
	separatorColor := CurrentThemeColor(cp.appState.App, theme.ColorNameSeparator)
	headerStack := container.NewVBox(headerBar, canvas.NewLine(separatorColor))

	// 左右双栏
	left := cp.buildLeftPanel(pad)
	right := cp.buildRightPanel(pad)
	split := container.NewHSplit(left, right)
	split.SetOffset(0.38)

	cp.content = container.NewBorder(
		headerStack,
		nil, nil, nil,
		newPaddedWithSize(split, pad),
	)
	return cp.content
}

// buildLeftPanel 构建左侧链面板。
func (cp *ChainPage) buildLeftPanel(pad float32) fyne.CanvasObject {
	cp.countLabel = widget.NewLabel("")
	cp.countLabel.Importance = widget.LowImportance

	cp.emptyHint = widget.NewLabel("从右侧拖拽节点到此处，首节点为入口、末节点为出口")
	cp.emptyHint.Wrapping = fyne.TextWrapWord
	cp.emptyHint.Importance = widget.LowImportance

	// 链行容器 + 插入指示线（指示线绝对定位在内容坐标内）
	cp.dropLine = canvas.NewRectangle(CurrentThemeColor(cp.appState.App, theme.ColorNamePrimary))
	cp.dropLine.CornerRadius = 2
	cp.dropLine.Hide()
	indicatorLayer := container.NewWithoutLayout(cp.dropLine)
	cp.chainBox = container.NewVBox()
	cp.renderChainList()
	content := container.NewStack(cp.chainBox, indicatorLayer)

	cp.chainListArea = container.NewScroll(content)
	cp.chainListArea.SetMinSize(fyne.NewSize(220, 0))

	header := container.NewHBox(NewTitleLabel("链"), layout.NewSpacer(), cp.countLabel)
	return container.NewBorder(
		container.NewVBox(header, cp.emptyHint, canvas.NewLine(CurrentThemeColor(cp.appState.App, theme.ColorNameSeparator))),
		nil, nil, nil,
		newPaddedWithSize(cp.chainListArea, pad),
	)
}

// buildRightPanel 构建右侧节点列表面板（搜索逻辑与节点页一致）。
func (cp *ChainPage) buildRightPanel(pad float32) fyne.CanvasObject {
	title := NewTitleLabel("节点列表")

	cp.searchEntry = widget.NewEntry()
	cp.searchEntry.SetPlaceHolder("搜索节点名称或地区...")
	cp.searchEntry.OnChanged = func(value string) {
		cp.searchText = strings.ToLower(strings.TrimSpace(value))
		cp.refreshNodeList()
	}
	cp.searchEntry.OnSubmitted = func(value string) {
		cp.searchText = strings.ToLower(strings.TrimSpace(value))
		cp.refreshNodeList()
	}
	searchBtn := widget.NewButtonWithIcon("", theme.SearchIcon(), func() {
		cp.searchText = strings.ToLower(strings.TrimSpace(cp.searchEntry.Text))
		cp.refreshNodeList()
	})
	searchBtn.Importance = widget.LowImportance

	cp.favoritesCheck = widget.NewCheck("仅收藏", func(on bool) {
		cp.favoritesOnly = on
		cp.refreshNodeList()
	})
	searchBar := container.NewBorder(
		nil, nil, nil,
		container.NewHBox(cp.favoritesCheck, searchBtn),
		cp.searchEntry,
	)

	hint := widget.NewLabel("拖拽节点到左侧链中（首=入口，末=出口）")
	hint.Wrapping = fyne.TextWrapWord
	hint.Importance = widget.LowImportance

	cp.nodeList = widget.NewList(
		cp.getNodeCount,
		cp.createNodeItem,
		cp.updateNodeItem,
	)
	cp.nodeScroll = container.NewScroll(cp.nodeList)

	return container.NewBorder(
		container.NewVBox(title, searchBar, hint, canvas.NewLine(CurrentThemeColor(cp.appState.App, theme.ColorNameSeparator))),
		nil, nil, nil,
		newPaddedWithSize(cp.nodeScroll, pad),
	)
}

// Refresh 刷新页面（进入页面时调用）。
func (cp *ChainPage) Refresh() {
	if cp == nil {
		return
	}
	// 若外部已保存过链，同步草稿（例如从设置页切换模式后返回）
	if cp.appState != nil && cp.appState.Store != nil && cp.appState.Store.Chain != nil {
		cp.draftIDs = cp.appState.Store.Chain.GetNodeIDs()
	}
	cp.renderChainList()
	cp.refreshNodeList()
}

// onClear 清空左侧链草稿。
func (cp *ChainPage) onClear() {
	if cp.appState == nil || cp.appState.Window == nil {
		return
	}
	if len(cp.draftIDs) == 0 {
		return
	}
	cp.appState.Dialogs.ShowConfirm("清空链式代理", "确定要清空当前链吗？未保存不会影响已保存的链。", func(ok bool) {
		if !ok {
			return
		}
		cp.draftIDs = make([]string, 0)
		cp.renderChainList()
	})
}

// onSave 保存链式代理配置：校验 >=2 个节点 → 持久化 → 切换链式模式 → 立即启动。
func (cp *ChainPage) onSave() {
	if cp.appState == nil || cp.appState.Window == nil {
		return
	}
	if cp.appState.Store == nil || cp.appState.Store.Chain == nil {
		cp.appState.Dialogs.ShowError(fmt.Errorf("链式代理存储未初始化"))
		return
	}
	if len(cp.draftIDs) < 2 {
		cp.appState.Dialogs.ShowInfo("无法保存", "链式代理至少需要 2 个节点，请先从右侧拖入节点")
		return
	}
	seen := make(map[string]bool, len(cp.draftIDs))
	for _, id := range cp.draftIDs {
		if seen[id] {
			cp.appState.Dialogs.ShowInfo("无法保存", "链中存在重复节点，请调整后重试")
			return
		}
		seen[id] = true
		if _, err := cp.appState.Store.Nodes.Get(id); err != nil {
			cp.appState.Dialogs.ShowError(fmt.Errorf("链中存在无效节点: %s", id))
			return
		}
	}

	if err := cp.appState.Store.Chain.SetNodeIDs(cp.draftIDs); err != nil {
		cp.appState.Dialogs.ShowError(err)
		return
	}
	if cp.appState.ConfigService != nil {
		if err := cp.appState.ConfigService.SetProxyChainMode(service.ProxyChainModeChain); err != nil {
			cp.appState.Dialogs.ShowError(err)
			return
		}
	}

	// 保存即生效：启动链式代理（StartProxy 会按模式分支；运行中的单节点代理会被替换）
	if cp.appState.MainWindow != nil {
		cp.appState.MainWindow.startProxyInternal(true)
	} else {
		cp.appState.Dialogs.ShowInfo("已保存", "链式代理配置已保存并启用")
	}
}

// getFilteredNodes 根据当前搜索关键字与收藏过滤返回节点列表（与节点页逻辑一致）。
func (cp *ChainPage) getFilteredNodes() []*model.Node {
	var allNodes []*model.Node
	if cp.appState != nil && cp.appState.Store != nil && cp.appState.Store.Nodes != nil {
		allNodes = cp.appState.Store.Nodes.GetAll()
	} else {
		allNodes = []*model.Node{}
	}
	filtered := make([]*model.Node, 0, len(allNodes))
	for _, node := range allNodes {
		if cp.favoritesOnly && !node.Favorited {
			continue
		}
		if cp.searchText == "" {
			filtered = append(filtered, node)
			continue
		}
		name := strings.ToLower(node.Name)
		addr := strings.ToLower(node.Addr)
		protocol := strings.ToLower(node.ProtocolType)
		if strings.Contains(name, cp.searchText) ||
			strings.Contains(addr, cp.searchText) ||
			strings.Contains(protocol, cp.searchText) {
			filtered = append(filtered, node)
		}
	}
	return filtered
}

// getNodeCount 返回过滤后的节点数量。
func (cp *ChainPage) getNodeCount() int {
	return len(cp.getFilteredNodes())
}

// createNodeItem 创建右侧节点列表项。
func (cp *ChainPage) createNodeItem() fyne.CanvasObject {
	return NewChainSourceItem(cp)
}

// updateNodeItem 更新右侧节点列表项。
func (cp *ChainPage) updateNodeItem(id widget.ListItemID, obj fyne.CanvasObject) {
	nodes := cp.getFilteredNodes()
	if id < 0 || id >= len(nodes) {
		return
	}
	item := obj.(*ChainSourceItem)
	item.Update(nodes[id])
}

// refreshNodeList 刷新右侧节点列表。
func (cp *ChainPage) refreshNodeList() {
	if cp.nodeList != nil {
		cp.nodeList.Refresh()
	}
}

// refreshChain 刷新左侧链并清除插入状态。
func (cp *ChainPage) refreshChain() {
	cp.dropIndex = -1
	cp.renderIndicator()
	cp.renderChainList()
}

// renderChainList 重建左侧链行列表。
func (cp *ChainPage) renderChainList() {
	if cp.chainBox == nil {
		return
	}
	cp.chainBox.RemoveAll()
	cp.chainRows = cp.chainRows[:0]
	for i, id := range cp.draftIDs {
		item := NewChainTargetItem(cp, id, i)
		cp.chainRows = append(cp.chainRows, item)
		cp.chainBox.Add(item)
	}
	if cp.countLabel != nil {
		cp.countLabel.SetText(fmt.Sprintf("共 %d 个节点", len(cp.draftIDs)))
	}
	if cp.emptyHint != nil {
		if len(cp.draftIDs) == 0 {
			cp.emptyHint.Show()
		} else {
			cp.emptyHint.Hide()
		}
	}
	cp.chainBox.Refresh()
}

// renderIndicator 更新左侧插入指示线的位置与可见性。
// 插入线位于链行之间（i 行上方 / 末行下方），按行实际位置计算，滚动时同样正确。
func (cp *ChainPage) renderIndicator() {
	if cp.dropLine == nil || cp.chainListArea == nil || cp.appState == nil || cp.appState.App == nil {
		return
	}
	if cp.dropIndex < 0 {
		cp.dropLine.Hide()
		cp.dropLine.Refresh()
		return
	}
	driver := cp.appState.App.Driver()
	contentTop := driver.AbsolutePositionForObject(cp.chainBox)
	var y float32
	if len(cp.chainRows) == 0 {
		y = 0
	} else if cp.dropIndex < len(cp.chainRows) {
		rowPos := driver.AbsolutePositionForObject(cp.chainRows[cp.dropIndex])
		y = rowPos.Y - contentTop.Y
	} else {
		last := cp.chainRows[len(cp.chainRows)-1]
		lastPos := driver.AbsolutePositionForObject(last)
		y = lastPos.Y + last.Size().Height - contentTop.Y
	}
	if y < 0 {
		y = 0
	}
	width := cp.chainListArea.Content.Size().Width
	if width <= 0 {
		width = cp.chainListArea.Size().Width
	}
	cp.dropLine.Move(fyne.NewPos(0, y))
	cp.dropLine.Resize(fyne.NewSize(width, 3))
	cp.dropLine.Show()
	cp.dropLine.Refresh()
}

// computeDropIndex 根据指针绝对位置计算左侧链插入下标；不在左侧区域返回 -1。
func (cp *ChainPage) computeDropIndex(pos fyne.Position) int {
	if cp.appState == nil || cp.appState.App == nil || cp.chainListArea == nil {
		return -1
	}
	driver := cp.appState.App.Driver()
	areaPos := driver.AbsolutePositionForObject(cp.chainListArea)
	areaSize := cp.chainListArea.Size()
	if pos.X < areaPos.X || pos.X > areaPos.X+areaSize.Width {
		return -1
	}
	if len(cp.chainRows) == 0 {
		if pos.Y >= areaPos.Y && pos.Y <= areaPos.Y+areaSize.Height {
			return 0
		}
		return -1
	}
	for i, row := range cp.chainRows {
		rowPos := driver.AbsolutePositionForObject(row)
		mid := rowPos.Y + row.Size().Height/2
		if pos.Y < mid {
			return i
		}
	}
	return len(cp.chainRows)
}

// beginDrag 记录拖拽起始状态。
func (cp *ChainPage) beginDrag(nodeID, nodeName string) {
	cp.dragNodeID = nodeID
	cp.dragNodeName = nodeName
}

// updateDrag 拖拽过程中更新悬浮矩形框与插入指示线。
func (cp *ChainPage) updateDrag(pos fyne.Position) {
	cp.lastDragPos = pos
	cp.ensureDragPopUp()
	if cp.dragPopUp != nil {
		cp.dragNameLabel.SetText(cp.dragNodeName)
		cp.dragPopUp.ShowAtPosition(pos.Add(fyne.NewPos(12, 12)))
	}
	idx := cp.computeDropIndex(pos)
	if idx != cp.dropIndex {
		cp.dropIndex = idx
		cp.renderIndicator()
	}
}

// ensureDragPopUp 创建悬浮矩形框（首次拖拽时）。
func (cp *ChainPage) ensureDragPopUp() {
	if cp.dragPopUp != nil || cp.appState == nil || cp.appState.Window == nil {
		return
	}
	card := canvas.NewRectangle(CurrentThemeColor(cp.appState.App, theme.ColorNameInputBackground))
	card.CornerRadius = 6
	card.StrokeColor = CurrentThemeColor(cp.appState.App, theme.ColorNamePrimary)
	card.StrokeWidth = 2
	cp.dragNameLabel = widget.NewLabel("")
	cp.dragNameLabel.TextStyle = fyne.TextStyle{Bold: true}
	cp.dragNameLabel.Truncation = fyne.TextTruncateEllipsis
	body := newPaddedWithSize(container.NewHBox(widget.NewIcon(theme.ListIcon()), cp.dragNameLabel), innerPadding(cp.appState))
	cp.dragPopUp = widget.NewPopUp(container.NewStack(card, body), cp.appState.Window.Canvas())
}

// hideDragPopUp 隐藏并释放悬浮矩形框。
func (cp *ChainPage) hideDragPopUp() {
	if cp.dragPopUp != nil {
		cp.dragPopUp.Hide()
		cp.dragPopUp = nil
	}
	cp.dragNameLabel = nil
}

// resetDragState 结束拖拽时清理状态（悬浮框、指示线、拖拽来源）。
func (cp *ChainPage) resetDragState() {
	cp.dragNodeID = ""
	cp.dragNodeName = ""
	cp.reorderSourceIdx = -1
	cp.hideDragPopUp()
	cp.dropIndex = -1
	cp.renderIndicator()
}

// endDragFromSource 从右侧节点列表拖入链。
func (cp *ChainPage) endDragFromSource() {
	defer cp.resetDragState()
	if cp.dragNodeID == "" {
		return
	}
	idx := cp.computeDropIndex(cp.lastDragPos)
	if idx < 0 {
		return
	}
	existing := -1
	for i, id := range cp.draftIDs {
		if id == cp.dragNodeID {
			existing = i
			break
		}
	}
	draft := append([]string{}, cp.draftIDs...)
	if existing >= 0 {
		// 已存在：移动到新位置
		draft = append(draft[:existing], draft[existing+1:]...)
		if idx > existing {
			idx--
		}
		cp.draftIDs = insertString(draft, idx, cp.dragNodeID)
	} else {
		cp.draftIDs = insertString(draft, idx, cp.dragNodeID)
	}
	cp.refreshChain()
}

// endDragReorder 左侧链内拖拽换序。
func (cp *ChainPage) endDragReorder() {
	defer cp.resetDragState()
	if cp.dragNodeID == "" || cp.reorderSourceIdx < 0 {
		return
	}
	src := cp.reorderSourceIdx
	if src >= len(cp.draftIDs) {
		return
	}
	idx := cp.computeDropIndex(cp.lastDragPos)
	if idx < 0 {
		return
	}
	draft := append([]string{}, cp.draftIDs...)
	nodeID := draft[src]
	draft = append(draft[:src], draft[src+1:]...)
	if idx > src {
		idx--
	}
	cp.draftIDs = insertString(draft, idx, nodeID)
	cp.refreshChain()
}

// insertString 在指定下标插入元素。
func insertString(list []string, idx int, val string) []string {
	if idx < 0 {
		idx = 0
	}
	if idx > len(list) {
		idx = len(list)
	}
	out := make([]string, 0, len(list)+1)
	out = append(out, list[:idx]...)
	out = append(out, val)
	out = append(out, list[idx:]...)
	return out
}

// ChainSourceItem 右侧节点列表项（可拖拽到左侧链）。
type ChainSourceItem struct {
	widget.BaseWidget
	page      *ChainPage
	node      *model.Node
	renderObj fyne.CanvasObject
	bgRect    *canvas.Rectangle
	nameLabel *widget.Label
	infoLabel *widget.Label
}

// NewChainSourceItem 创建右侧可拖拽节点项。
func NewChainSourceItem(page *ChainPage) *ChainSourceItem {
	item := &ChainSourceItem{page: page}
	item.bgRect = canvas.NewRectangle(CurrentThemeColor(page.appState.App, theme.ColorNameInputBackground))
	item.bgRect.CornerRadius = 4
	item.nameLabel = widget.NewLabel("")
	item.nameLabel.Wrapping = fyne.TextTruncate
	item.nameLabel.TextStyle = fyne.TextStyle{Bold: true}
	item.infoLabel = widget.NewLabel("")
	item.infoLabel.Wrapping = fyne.TextTruncate
	item.infoLabel.Importance = widget.LowImportance
	content := container.NewBorder(nil, nil, nil, item.infoLabel, item.nameLabel)
	item.renderObj = container.NewStack(item.bgRect, newPaddedWithSize(content, innerPadding(page.appState)))
	item.ExtendBaseWidget(item)
	return item
}

// Update 更新节点项内容。
func (si *ChainSourceItem) Update(node *model.Node) {
	si.node = node
	if node == nil {
		return
	}
	si.nameLabel.SetText(node.Name)
	si.infoLabel.SetText(fmt.Sprintf("%s:%d · %s", node.Addr, node.Port, node.ProtocolType))
}

// MinSize 返回节点项最小尺寸（统一行高）。
func (si *ChainSourceItem) MinSize() fyne.Size {
	return fyne.NewSize(0, chainRowHeight)
}

// CreateRenderer 创建渲染器。
func (si *ChainSourceItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(si.renderObj)
}

// Dragged 拖拽中：更新悬浮框与插入指示线。
func (si *ChainSourceItem) Dragged(ev *fyne.DragEvent) {
	if si.page == nil || si.node == nil {
		return
	}
	cp := si.page
	cp.beginDrag(si.node.ID, si.node.Name)
	cp.updateDrag(ev.AbsolutePosition)
}

// DragEnd 拖拽结束：并入左侧链。
func (si *ChainSourceItem) DragEnd() {
	if si.page != nil {
		si.page.endDragFromSource()
	}
}

// ChainTargetItem 左侧链节点行（可拖拽换序、移除）。
type ChainTargetItem struct {
	widget.BaseWidget
	page      *ChainPage
	nodeID    string
	index     int
	renderObj fyne.CanvasObject
	bgRect    *canvas.Rectangle
	idxLabel  *widget.Label
	nameLabel *widget.Label
	removeBtn *widget.Button
}

// NewChainTargetItem 创建左侧链节点行。
func NewChainTargetItem(page *ChainPage, nodeID string, index int) *ChainTargetItem {
	item := &ChainTargetItem{page: page, nodeID: nodeID, index: index}
	item.bgRect = canvas.NewRectangle(CurrentThemeColor(page.appState.App, theme.ColorNameInputBackground))
	item.bgRect.CornerRadius = 4
	item.idxLabel = widget.NewLabel(fmt.Sprintf("%d", index+1))
	item.idxLabel.Alignment = fyne.TextAlignCenter
	item.idxLabel.TextStyle = fyne.TextStyle{Bold: true}
	item.idxLabel.Importance = widget.MediumImportance

	item.nameLabel = widget.NewLabel("")
	item.nameLabel.Wrapping = fyne.TextTruncate
	item.nameLabel.TextStyle = fyne.TextStyle{Bold: true}

	item.removeBtn = widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		page.removeNode(nodeID)
	})
	item.removeBtn.Importance = widget.LowImportance

	content := container.NewBorder(nil, nil, nil, item.removeBtn,
		container.NewHBox(item.idxLabel, item.nameLabel))
	item.renderObj = container.NewStack(item.bgRect, newPaddedWithSize(content, innerPadding(page.appState)))
	item.ExtendBaseWidget(item)

	if page.appState != nil && page.appState.Store != nil && page.appState.Store.Nodes != nil {
		if node, err := page.appState.Store.Nodes.Get(nodeID); err == nil {
			item.nameLabel.SetText(node.Name)
		} else {
			item.nameLabel.SetText(nodeID)
			item.nameLabel.Importance = widget.LowImportance
		}
	}
	return item
}

// MinSize 返回链节点行最小尺寸。
func (ti *ChainTargetItem) MinSize() fyne.Size {
	return fyne.NewSize(0, chainRowHeight)
}

// CreateRenderer 创建渲染器。
func (ti *ChainTargetItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(ti.renderObj)
}

// Dragged 拖拽中：更新悬浮框与插入指示线，并记录换序源下标。
func (ti *ChainTargetItem) Dragged(ev *fyne.DragEvent) {
	if ti.page == nil {
		return
	}
	cp := ti.page
	name := ti.nodeID
	if node, err := cp.appState.Store.Nodes.Get(ti.nodeID); err == nil {
		name = node.Name
	}
	cp.beginDrag(ti.nodeID, name)
	cp.reorderSourceIdx = ti.index
	cp.updateDrag(ev.AbsolutePosition)
}

// DragEnd 拖拽结束：链内换序。
func (ti *ChainTargetItem) DragEnd() {
	if ti.page != nil {
		ti.page.endDragReorder()
	}
}

// removeNode 从链草稿移除节点。
func (cp *ChainPage) removeNode(nodeID string) {
	for i, id := range cp.draftIDs {
		if id == nodeID {
			cp.draftIDs = append(cp.draftIDs[:i], cp.draftIDs[i+1:]...)
			break
		}
	}
	cp.refreshChain()
}
