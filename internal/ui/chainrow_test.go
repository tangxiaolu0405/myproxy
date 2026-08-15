package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"myproxy.com/p/internal/database"
	"myproxy.com/p/internal/model"
	"myproxy.com/p/internal/store"
)

// TestChainTargetItemNameVisible 回归测试：链行名称标签必须获得显式宽度。
// 截断 Label（TextTruncate）的 MinSize 仅为「一个字符」宽，若放入按 MinSize 布局的 HBox，
// 名称会被压到不可见；行布局必须给名称分配剩余宽度。
func TestChainTargetItemNameVisible(t *testing.T) {
	if err := database.InitDB(":memory:"); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	app := test.NewApp()
	as := NewAppState("test")
	as.App = app
	as.Store = store.NewStore(nil)
	if err := as.Store.Nodes.Add(&model.Node{ID: "n1", Name: "【直连】香港Microsoft", Addr: "a.com", Port: 443, ProtocolType: "socks5"}); err != nil {
		t.Fatalf("add node: %v", err)
	}

	cp := NewChainPage(as)
	cp.draftIDs = []string{"n1"}
	item := NewChainTargetItem(cp, "n1", 0)
	if item.nameLabel.Text != "【直连】香港Microsoft" {
		t.Fatalf("nameLabel.Text=%q want node name", item.nameLabel.Text)
	}

	w := test.NewWindow(item)
	w.Resize(fyne.NewSize(200, 44))
	w.Show()
	item.Resize(fyne.NewSize(200, 44))
	app.Driver().CanvasForObject(item).Refresh(item)

	// 在 200px 行宽下，名称标签应占明显宽度（而非截断 Label 的极小 MinSize）
	if item.nameLabel.Size().Width < 100 {
		t.Fatalf("nameLabel width=%v too small (name would be invisible)", item.nameLabel.Size().Width)
	}
	// 删除按钮应在最右侧
	delRight := item.removeBtn.Position().X + item.removeBtn.Size().Width
	if delRight > item.Size().Width+0.5 {
		t.Fatalf("removeBtn overflows row: right=%v rowW=%v", delRight, item.Size().Width)
	}
}
