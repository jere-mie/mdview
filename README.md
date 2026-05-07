# mdview

`mdview` is a Go CLI for live-previewing Markdown in a browser, browsing directories, and exporting standalone HTML files.

## Features

- GitHub-styled Markdown rendering with syntax highlighting
- Mermaid diagram rendering with vendored assets
- Live reload over WebSockets when watched files change
- Directory browser that renders Markdown and serves other files directly
- Standalone HTML conversion with embedded CSS
- Embedded version string from `version.txt`

## Usage

```powershell
mdview [path]
mdview -p 9090 docs
MDVIEW_PORT=9091 mdview docs
mdview -H 0.0.0.0 docs
mdview -c README.md
mdview -c README.md -o README.preview.html
mdview -v
```

## Flags

| Flag | Description |
| --- | --- |
| `-H`, `--host` | Server host. Defaults to `127.0.0.1`. |
| `-p`, `--port` | Server port. Defaults to `8080` unless `MDVIEW_PORT` is set. |
| `-c`, `--convert` | Convert a Markdown file to standalone HTML instead of starting the server. |
| `-o`, `--output` | Output file path for conversion mode. |
| `-v`, `--version` | Print the embedded version string. |

## Behavior

- Passing a Markdown file starts file mode and watches that file for reloads.
- Passing a directory starts directory mode and serves a file browser rooted at that path.
- Markdown files are rendered as HTML pages, while other files are served directly.
- If the requested port is already in use, mdview probes the next ports until it finds a free one or gives up after 1000 attempts.
- Conversion mode writes a self-contained HTML file with embedded styling.

## Development

```powershell
go test ./...
go run . .
```

## Third-party notices

`internal/assets/github-markdown.css` is derived from `github-markdown-css` v5.5.1, and `internal/assets/mermaid.min.js` is bundled from `mermaid` v11.12.0. Both are MIT licensed. See `THIRD_PARTY_NOTICES.txt` for the bundled attribution summary.
