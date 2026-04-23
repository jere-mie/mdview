package renderer

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/jere-mie/mdview/internal/assets"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

type Renderer struct {
	markdown goldmark.Markdown
	pageTmpl *template.Template
	pageCSS  template.CSS
}

type Options struct {
	Title          string
	LiveReload     bool
	ReloadEndpoint string
}

type pageData struct {
	Title        string
	CSS          template.CSS
	Content      template.HTML
	ReloadScript template.HTML
}

func New() *Renderer {
	pageCSS, err := buildPageCSS()
	if err != nil {
		panic(err)
	}

	return &Renderer{
		markdown: goldmark.New(
			goldmark.WithExtensions(
				extension.GFM,
				highlighting.NewHighlighting(
					highlighting.WithStyle("github"),
					highlighting.WithGuessLanguage(true),
					highlighting.WithFormatOptions(
						chromahtml.WithClasses(true),
					),
				),
			),
			goldmark.WithParserOptions(
				parser.WithAutoHeadingID(),
			),
			goldmark.WithRendererOptions(
				gmhtml.WithUnsafe(),
			),
		),
		pageTmpl: template.Must(template.New("page").Parse(assets.PageTemplate())),
		pageCSS:  template.CSS(pageCSS),
	}
}

func (r *Renderer) Render(content []byte, opts Options) ([]byte, error) {
	var rendered bytes.Buffer
	if err := r.markdown.Convert(content, &rendered); err != nil {
		return nil, fmt.Errorf("convert markdown: %w", err)
	}

	reloadScript := ""
	if opts.LiveReload {
		endpoint := opts.ReloadEndpoint
		if endpoint == "" {
			endpoint = "/ws"
		}
		reloadScript = liveReloadScript(endpoint)
	}

	data := pageData{
		Title:        defaultString(opts.Title, "mdview"),
		CSS:          r.pageCSS,
		Content:      template.HTML(rendered.String()),
		ReloadScript: template.HTML(reloadScript),
	}

	var output bytes.Buffer
	if err := r.pageTmpl.Execute(&output, data); err != nil {
		return nil, fmt.Errorf("render HTML page: %w", err)
	}

	return output.Bytes(), nil
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func buildPageCSS() (string, error) {
	highlightCSS, err := buildHighlightCSS()
	if err != nil {
		return "", err
	}

	return strings.Join([]string{
		assets.GitHubMarkdownCSS(),
		highlightCSS,
	}, "\n\n"), nil
}

func buildHighlightCSS() (string, error) {
	themes := []struct {
		name      string
		styleName string
	}{
		{name: "light", styleName: "github"},
		{name: "dark", styleName: "github-dark"},
	}

	blocks := make([]string, 0, len(themes))
	for _, theme := range themes {
		style := styles.Get(theme.styleName)
		if style == nil {
			return "", fmt.Errorf("highlight style %q not found", theme.styleName)
		}

		formatter := chromahtml.New(chromahtml.WithClasses(true))

		var css bytes.Buffer
		if err := formatter.WriteCSS(&css, style); err != nil {
			return "", fmt.Errorf("write highlight CSS for %s: %w", theme.styleName, err)
		}

		prefix := fmt.Sprintf(`:root[data-theme="%s"] .markdown-body `, theme.name)
		blocks = append(blocks, strings.ReplaceAll(css.String(), ".chroma", prefix+".chroma"))
	}

	return strings.Join(blocks, "\n\n"), nil
}

func liveReloadScript(endpoint string) string {
	return fmt.Sprintf(`<script>
(function () {
  var timer = null;

  function connect() {
    var scheme = window.location.protocol === "https:" ? "wss://" : "ws://";
    var socket = new WebSocket(scheme + window.location.host + %q);

    socket.onmessage = function (event) {
      if (event.data === "reload") {
        window.location.reload();
      }
    };

    socket.onclose = function () {
      timer = window.setTimeout(connect, 500);
    };

    socket.onerror = function () {
      socket.close();
    };
  }

  connect();
})();
</script>`, endpoint)
}
