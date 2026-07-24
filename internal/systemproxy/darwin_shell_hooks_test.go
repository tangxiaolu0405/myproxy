package systemproxy

import "testing"

func TestStripMyproxyShellHooks_RemovesOrphanComments(t *testing.T) {
	in := `# user stuff
export FOO=1
# Source myproxy proxy settings
# Source myproxy proxy settings
# Source myproxy proxy settings
source /Users/me/.myproxy_proxy.sh
# more stuff
`
	got := stripMyproxyShellHooks(in)
	want := `# user stuff
export FOO=1
# more stuff
`
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestStripMyproxyShellHooks_IdempotentSetupShape(t *testing.T) {
	home := "/Users/me"
	sourceLine := "source " + home + "/.myproxy_proxy.sh"
	base := "# user stuff\n"

	// 模拟多次 setup：每次 strip 后再追加一块
	content := base
	for i := 0; i < 3; i++ {
		cleaned := stripMyproxyShellHooks(content)
		content = cleaned
		if len(content) > 0 && content[len(content)-1] != '\n' {
			content += "\n"
		}
		content += myproxyShellMarker + "\n" + sourceLine + "\n"
	}

	markerCount := 0
	sourceCount := 0
	for _, line := range splitLines(content) {
		if line == myproxyShellMarker {
			markerCount++
		}
		if line == sourceLine {
			sourceCount++
		}
	}
	if markerCount != 1 || sourceCount != 1 {
		t.Fatalf("marker=%d source=%d content:\n%s", markerCount, sourceCount, content)
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
