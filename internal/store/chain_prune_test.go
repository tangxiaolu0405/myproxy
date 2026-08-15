package store

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"myproxy.com/p/internal/database"
	"myproxy.com/p/internal/model"
)

// setupChainPruneStore 初始化内存数据库 + Fyne 应用上下文（节点 binding 需要）。
func setupChainPruneStore(t *testing.T) *Store {
	t.Helper()
	if err := database.InitDB(":memory:"); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	test.NewApp()
	return NewStore(nil)
}

// TestChainPruneInvalid 链中残留失效节点 ID 时，加载后应被清理并持久化。
func TestChainPruneInvalid(t *testing.T) {
	st := setupChainPruneStore(t)
	for _, n := range []*model.Node{
		{ID: "n1", Name: "N1", Addr: "1.1.1.1", Port: 1, ProtocolType: "socks5"},
		{ID: "n2", Name: "N2", Addr: "2.2.2.2", Port: 2, ProtocolType: "socks5"},
	} {
		if err := st.Nodes.Add(n); err != nil {
			t.Fatalf("Add node: %v", err)
		}
	}
	// 链中混入一个不存在的失效 ID（订阅刷新后残留）
	if err := st.Chain.SetNodeIDs([]string{"n1", "stale-id", "n2"}); err != nil {
		t.Fatalf("SetNodeIDs: %v", err)
	}
	removed := st.Chain.PruneInvalid(st.Nodes)
	if removed != 1 {
		t.Fatalf("removed=%d want 1", removed)
	}
	got := st.Chain.GetNodeIDs()
	if len(got) != 2 || got[0] != "n1" || got[1] != "n2" {
		t.Fatalf("chain after prune=%v want [n1 n2]", got)
	}
	// 校验已持久化
	raw, err := database.GetAppConfig("proxyChain")
	if err != nil || raw != `["n1","n2"]` {
		t.Fatalf("persisted proxyChain=%q err=%v", raw, err)
	}
}

// TestChainPruneInvalidAllInvalid 链全部失效时清理为空。
func TestChainPruneInvalidAllInvalid(t *testing.T) {
	st := setupChainPruneStore(t)
	if err := st.Nodes.Add(&model.Node{ID: "n1", Name: "N1", Addr: "1.1.1.1", Port: 1, ProtocolType: "socks5"}); err != nil {
		t.Fatalf("Add node: %v", err)
	}
	if err := st.Chain.SetNodeIDs([]string{"gone-1", "gone-2"}); err != nil {
		t.Fatalf("SetNodeIDs: %v", err)
	}
	if removed := st.Chain.PruneInvalid(st.Nodes); removed != 2 {
		t.Fatalf("removed=%d want 2", removed)
	}
	if got := st.Chain.GetNodeIDs(); len(got) != 0 {
		t.Fatalf("chain after prune=%v want empty", got)
	}
}

// TestChainPruneInvalidNoChange 链全部有效时不做任何改动。
func TestChainPruneInvalidNoChange(t *testing.T) {
	st := setupChainPruneStore(t)
	for _, n := range []*model.Node{
		{ID: "n1", Name: "N1", Addr: "1.1.1.1", Port: 1, ProtocolType: "socks5"},
		{ID: "n2", Name: "N2", Addr: "2.2.2.2", Port: 2, ProtocolType: "socks5"},
	} {
		if err := st.Nodes.Add(n); err != nil {
			t.Fatalf("Add node: %v", err)
		}
	}
	if err := st.Chain.SetNodeIDs([]string{"n1", "n2"}); err != nil {
		t.Fatalf("SetNodeIDs: %v", err)
	}
	if removed := st.Chain.PruneInvalid(st.Nodes); removed != 0 {
		t.Fatalf("removed=%d want 0", removed)
	}
	if got := st.Chain.GetNodeIDs(); len(got) != 2 {
		t.Fatalf("chain after prune=%v want [n1 n2]", got)
	}
}
