package subscription

import (
	"testing"

	"myproxy.com/p/internal/model"
	"myproxy.com/p/internal/utils"
)

// TestGenerateServerIDDeterministic 同一服务器生成的 ID 必须稳定（订阅刷新不会更换节点 ID）。
func TestGenerateServerIDDeterministic(t *testing.T) {
	a := utils.GenerateServerID("example.com", 443, "uuid-1")
	b := utils.GenerateServerID("example.com", 443, "uuid-1")
	if a != b {
		t.Fatalf("same server got different IDs: %s vs %s", a, b)
	}
	if a == utils.GenerateServerID("example.com", 443, "uuid-2") {
		t.Fatalf("different username should yield different ID")
	}
	if a == utils.GenerateServerID("example.com", 8443, "uuid-1") {
		t.Fatalf("different port should yield different ID")
	}
}

// TestReuseExistingIDs 刷新后同一 (addr:port:username) 的服务器沿用旧 ID。
func TestReuseExistingIDs(t *testing.T) {
	idByKey := map[string]string{
		"a.com:443:u1": "old-id-1",
		"a.com:443:u2": "old-id-2",
	}
	servers := []model.Node{
		{ID: "new-1", Name: "S1", Addr: "a.com", Port: 443, Username: "u1"},
		{ID: "new-2", Name: "S2", Addr: "a.com", Port: 443, Username: "u2"},
		{ID: "new-3", Name: "S3", Addr: "b.com", Port: 443, Username: "u3"}, // 新服务器，无旧 ID
	}
	reuseExistingIDs(servers, idByKey)
	if servers[0].ID != "old-id-1" {
		t.Fatalf("servers[0].ID=%s want old-id-1", servers[0].ID)
	}
	if servers[1].ID != "old-id-2" {
		t.Fatalf("servers[1].ID=%s want old-id-2", servers[1].ID)
	}
	if servers[2].ID != "new-3" {
		t.Fatalf("servers[2].ID=%s want new-3 (unchanged)", servers[2].ID)
	}
}

// TestServerIdentityKey 身份键由 addr:port:username 组成。
func TestServerIdentityKey(t *testing.T) {
	k := serverIdentityKey(&model.Node{Addr: "a.com", Port: 8080, Username: "u"})
	if k != "a.com:8080:u" {
		t.Fatalf("key=%q", k)
	}
}
