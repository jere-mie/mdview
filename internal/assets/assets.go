package assets

import _ "embed"

var (
	//go:embed github-markdown.css
	githubMarkdownCSS string

	//go:embed templates/page.html
	pageTemplate string

	//go:embed templates/index.html
	indexTemplate string
)

func GitHubMarkdownCSS() string {
	return githubMarkdownCSS
}

func PageTemplate() string {
	return pageTemplate
}

func IndexTemplate() string {
	return indexTemplate
}
