package xray

import (
	"encoding/json"
	"testing"

	"myproxy.com/p/internal/model"
)

func chainTestNode(id, name, addr string, port int, proto string) *model.Node {
	return &model.Node{ID: id, Name: name, Addr: addr, Port: port, ProtocolType: proto}
}

// TestCreateChainXrayConfig_Empty 空节点列表应报错。
func TestCreateChainXrayConfig_Empty(t *testing.T) {
	if _, err := CreateChainXrayConfig(10808, "127.0.0.1", nil, "", nil, false); err == nil {
		t.Fatal("expected error for empty node list")
	}
}

// TestCreateChainXrayConfig_SingleNodeFallsBack 单节点应退化为普通配置（出站仅 proxy + direct）。
func TestCreateChainXrayConfig_SingleNodeFallsBack(t *testing.T) {
	data, err := CreateChainXrayConfig(10808, "127.0.0.1",
		[]*model.Node{chainTestNode("n1", "N1", "1.2.3.4", 443, "vmess")}, "", nil, false)
	if err != nil {
		t.Fatalf("CreateChainXrayConfig error: %v", err)
	}
	var cfg struct {
		Outbounds []map[string]interface{} `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(cfg.Outbounds) != 2 {
		t.Fatalf("outbounds len=%d want 2", len(cfg.Outbounds))
	}
	if cfg.Outbounds[0]["tag"] != "proxy" {
		t.Fatalf("outbound[0] tag=%v want proxy", cfg.Outbounds[0]["tag"])
	}
	if _, ok := cfg.Outbounds[0]["proxySettings"]; ok {
		t.Fatal("single-node outbound should not carry proxySettings")
	}
	if cfg.Outbounds[1]["tag"] != "direct" {
		t.Fatalf("outbound[1] tag=%v want direct", cfg.Outbounds[1]["tag"])
	}
}

// TestCreateChainXrayConfig_ThreeNodes 三节点链：中间节点按位置命名并携带 proxySettings.tag 链式拨号。
func TestCreateChainXrayConfig_ThreeNodes(t *testing.T) {
	nodes := []*model.Node{
		chainTestNode("n1", "N1", "1.2.3.4", 443, "socks5"),
		chainTestNode("n2", "N2", "5.6.7.8", 1080, "socks5"),
		chainTestNode("n3", "N3", "9.9.9.9", 8388, "socks5"),
	}
	data, err := CreateChainXrayConfig(10808, "127.0.0.1", nodes, "", nil, false)
	if err != nil {
		t.Fatalf("CreateChainXrayConfig error: %v", err)
	}
	var cfg struct {
		Outbounds []map[string]interface{} `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(cfg.Outbounds) != 4 { // 3 个链节点 + direct
		t.Fatalf("outbounds len=%d want 4", len(cfg.Outbounds))
	}
	expects := []struct {
		tag          string
		proxySetting string // 空表示不应有 proxySettings
	}{
		{"chain-0", ""},
		{"chain-1", "chain-0"},
		{"proxy", "chain-1"},
	}
	for i, e := range expects {
		ob := cfg.Outbounds[i]
		if ob["tag"] != e.tag {
			t.Fatalf("outbound[%d] tag=%v want %s", i, ob["tag"], e.tag)
		}
		ps, ok := ob["proxySettings"]
		if e.proxySetting == "" {
			if ok {
				t.Fatalf("outbound[%d] unexpected proxySettings=%v", i, ps)
			}
			continue
		}
		if !ok {
			t.Fatalf("outbound[%d] missing proxySettings", i)
		}
		psMap, ok := ps.(map[string]interface{})
		if !ok {
			t.Fatalf("outbound[%d] proxySettings type=%T", i, ps)
		}
		if psMap["tag"] != e.proxySetting {
			t.Fatalf("outbound[%d] proxySettings.tag=%v want %s", i, psMap["tag"], e.proxySetting)
		}
	}
	if cfg.Outbounds[3]["tag"] != "direct" {
		t.Fatalf("outbound[3] tag=%v want direct", cfg.Outbounds[3]["tag"])
	}
}
