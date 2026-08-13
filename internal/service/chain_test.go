package service

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"myproxy.com/p/internal/database"
	"myproxy.com/p/internal/model"
	"myproxy.com/p/internal/store"
)

// setupChainNodes 初始化内存数据库并写入测试节点。
func setupChainNodes(t *testing.T) *store.Store {
	t.Helper()
	if err := database.InitDB(":memory:"); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	// 节点 Store 的 binding 需要 Fyne 应用上下文
	test.NewApp()
	t.Cleanup(func() {})
	st := store.NewStore(nil)
	for _, n := range []*model.Node{
		{ID: "n1", Name: "N1", Addr: "1.1.1.1", Port: 443, ProtocolType: "socks5"},
		{ID: "n2", Name: "N2", Addr: "2.2.2.2", Port: 443, ProtocolType: "socks5"},
		{ID: "n3", Name: "N3", Addr: "3.3.3.3", Port: 443, ProtocolType: "socks5"},
	} {
		if err := st.Nodes.Add(n); err != nil {
			t.Fatalf("Add node: %v", err)
		}
	}
	return st
}

// TestResolveChainNodes 正常链按顺序解析。
func TestResolveChainNodes(t *testing.T) {
	st := setupChainNodes(t)

	nodes, err := resolveChainNodes([]string{"n1", "n2", "n3"}, st)
	if err != nil {
		t.Fatalf("resolveChainNodes error: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("nodes len=%d want 3", len(nodes))
	}
	if nodes[0].ID != "n1" || nodes[1].ID != "n2" || nodes[2].ID != "n3" {
		t.Fatalf("unexpected node order: %s,%s,%s", nodes[0].ID, nodes[1].ID, nodes[2].ID)
	}
}

// TestResolveChainNodesErrors 少于 2 个节点 / 重复节点 / 节点不存在均应报错。
func TestResolveChainNodesErrors(t *testing.T) {
	st := setupChainNodes(t)

	cases := [][]string{
		{"n1"},           // 少于 2 个节点
		{"n1", "n1"},     // 重复节点
		{"n1", "nope"},   // 节点不存在
		{"n1", "n2", ""}, // 空节点 ID
	}
	for _, ids := range cases {
		if _, err := resolveChainNodes(ids, st); err == nil {
			t.Fatalf("expected error for ids=%v", ids)
		}
	}
}
