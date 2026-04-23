package renderer

import (
	"strings"
	"testing"
)

func TestRenderStandaloneHTML(t *testing.T) {
	renderer := New()

	output, err := renderer.Render([]byte("# Title\n\n```go\nfmt.Println(\"hi\")\n```"), Options{
		Title: "Example",
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	html := string(output)
	checks := []string{
		"<title>Example</title>",
		`class="markdown-body"`,
		"<h1 id=\"title\">Title</h1>",
		"Println",
		`data-theme-toggle`,
		`class="chroma"`,
	}

	for _, check := range checks {
		if !strings.Contains(html, check) {
			t.Fatalf("expected rendered HTML to contain %q", check)
		}
	}

	if strings.Contains(html, "ZgotmplZ") {
		t.Fatalf("expected embedded CSS to remain intact")
	}
}

func TestRenderIncludesLiveReloadWhenEnabled(t *testing.T) {
	renderer := New()

	output, err := renderer.Render([]byte("content"), Options{
		LiveReload:     true,
		ReloadEndpoint: "/ws",
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	html := string(output)
	if !strings.Contains(html, "new WebSocket") {
		t.Fatalf("expected live reload script to be present")
	}
	if !strings.Contains(html, `"/ws"`) {
		t.Fatalf("expected websocket endpoint to be embedded")
	}
}
