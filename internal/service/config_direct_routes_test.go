package service

import "testing"

func TestNormalizeDirectRouteInput(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"bilibili.com", "domain:bilibili.com"},
		{"bilibili", "keyword:bilibili"},
		{"domain:bilibili.com", "domain:bilibili.com"},
		{"keyword:foo", "keyword:foo"},
		{"10.0.0.0/8", "10.0.0.0/8"},
		{"127.0.0.1", "127.0.0.1"},
	}
	for _, c := range cases {
		got := NormalizeDirectRouteInput(c.in)
		if len(got) != 1 || got[0] != c.want {
			t.Fatalf("in=%q got=%v want=[%q]", c.in, got, c.want)
		}
	}
}
