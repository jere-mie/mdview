package renderer

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/jere-mie/mdview/internal/assets"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmrenderer "github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	gmutil "github.com/yuin/goldmark/util"
)

var mermaidFencePattern = regexp.MustCompile("(?mi)^[ \\t]{0,3}(?:```+|~~~+)[ \\t]*mermaid(?:[ \\t]+[^\\r\\n]*)?\\r?$")

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
	Title         string
	CSS           template.CSS
	Content       template.HTML
	MermaidBundle template.JS
	MermaidInit   template.JS
	ReloadScript  template.HTML
}

func New() *Renderer {
	pageCSS, err := buildPageCSS()
	if err != nil {
		panic(err)
	}

	highlightRenderer := highlighting.NewHTMLRenderer(
		highlighting.WithStyle("github"),
		highlighting.WithGuessLanguage(true),
		highlighting.WithFormatOptions(
			chromahtml.WithClasses(true),
		),
	)

	return &Renderer{
		markdown: goldmark.New(
			goldmark.WithExtensions(
				extension.GFM,
				newMermaidExtender(highlightRenderer),
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

	hasMermaid := containsMermaidFence(content)

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
	if hasMermaid {
		data.MermaidBundle = template.JS(assets.MermaidJS())
		data.MermaidInit = template.JS(mermaidInitScript())
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
		mermaidCSS(),
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

func containsMermaidFence(content []byte) bool {
	return mermaidFencePattern.Match(content)
}

func mermaidCSS() string {
	return `
.markdown-body .mermaid {
	display: block;
	margin: 0 0 16px;
	padding: 16px;
	overflow-x: auto;
	border-radius: 6px;
	background: var(--color-canvas-subtle);
}

.markdown-body .mermaid svg {
	display: block;
	max-width: 100%;
	height: auto;
	margin: 0 auto;
}

.markdown-body .mermaid[data-mermaid-error="true"] {
	white-space: pre;
	font-family: ui-monospace, SFMono-Regular, SF Mono, Menlo, Consolas, Liberation Mono, monospace;
}
`
}

func mermaidInitScript() string {
	return `
(function () {
	var sourceAttribute = "data-mermaid-source";
	var renderCounter = 0;

	function mermaidTheme() {
		return document.documentElement.getAttribute("data-theme") === "dark" ? "dark" : "default";
	}

	async function renderMermaidDiagrams() {
		if (!window.mermaid) {
			return;
		}

		window.mermaid.initialize({
			startOnLoad: false,
			securityLevel: "loose",
			theme: mermaidTheme()
		});

		var diagrams = Array.from(document.querySelectorAll(".mermaid"));
		for (var i = 0; i < diagrams.length; i += 1) {
			var node = diagrams[i];
			if (!node.getAttribute(sourceAttribute)) {
				node.setAttribute(sourceAttribute, node.textContent || "");
			}

			var source = node.getAttribute(sourceAttribute) || "";
			if (!source.trim()) {
				continue;
			}

			try {
				var result = await window.mermaid.render("mdview-mermaid-" + renderCounter, source);
				renderCounter += 1;
				node.innerHTML = result.svg;
				node.removeAttribute("data-mermaid-error");
				if (typeof result.bindFunctions === "function") {
					result.bindFunctions(node);
				}
			} catch (error) {
				node.textContent = source;
				node.setAttribute("data-mermaid-error", "true");
				console.error("Failed to render Mermaid diagram", error);
			}
		}
	}

	window.__mdviewRenderMermaid = function () {
		void renderMermaidDiagrams();
	};

	if (document.readyState === "loading") {
		document.addEventListener("DOMContentLoaded", function () {
			void renderMermaidDiagrams();
		}, { once: true });
		return;
	}

	void renderMermaidDiagrams();
})();
`
}

type mermaidExtender struct {
	renderer gmrenderer.NodeRenderer
}

func newMermaidExtender(fallback gmrenderer.NodeRenderer) goldmark.Extender {
	return &mermaidExtender{
		renderer: newMermaidFencedCodeBlockRenderer(fallback),
	}
}

func (e *mermaidExtender) Extend(markdown goldmark.Markdown) {
	markdown.Renderer().AddOptions(gmrenderer.WithNodeRenderers(
		gmutil.Prioritized(e.renderer, 100),
	))
}

type mermaidFencedCodeBlockRenderer struct {
	fallback gmrenderer.NodeRendererFunc
}

type nodeRendererFuncMap map[ast.NodeKind]gmrenderer.NodeRendererFunc

func (m nodeRendererFuncMap) Register(kind ast.NodeKind, fn gmrenderer.NodeRendererFunc) {
	m[kind] = fn
}

func newMermaidFencedCodeBlockRenderer(fallback gmrenderer.NodeRenderer) gmrenderer.NodeRenderer {
	funcs := nodeRendererFuncMap{}
	fallback.RegisterFuncs(funcs)

	return &mermaidFencedCodeBlockRenderer{
		fallback: funcs[ast.KindFencedCodeBlock],
	}
}

func (r *mermaidFencedCodeBlockRenderer) RegisterFuncs(reg gmrenderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCodeBlock)
}

func (r *mermaidFencedCodeBlockRenderer) renderFencedCodeBlock(w gmutil.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	block := node.(*ast.FencedCodeBlock)
	if !bytes.EqualFold(block.Language(source), []byte("mermaid")) {
		return r.fallback(w, source, node, entering)
	}

	if !entering {
		return ast.WalkContinue, nil
	}

	_, _ = w.WriteString("<pre class=\"mermaid\">")
	for i := range block.Lines().Len() {
		line := block.Lines().At(i)
		_, _ = w.WriteString(html.EscapeString(string(line.Value(source))))
	}
	_, _ = w.WriteString("</pre>\n")

	return ast.WalkSkipChildren, nil
}
