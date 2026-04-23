package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/jere-mie/mdview/internal/renderer"
	"github.com/jere-mie/mdview/internal/server"
)

//go:embed version.txt
var version string

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "mdview: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("mdview", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	var (
		port        int
		host        string
		convertMode bool
		outputPath  string
		showVersion bool
	)

	flags.IntVar(&port, "p", 8080, "server port")
	flags.IntVar(&port, "port", 8080, "server port")
	flags.StringVar(&host, "H", "127.0.0.1", "server host")
	flags.StringVar(&host, "host", "127.0.0.1", "server host")
	flags.BoolVar(&convertMode, "c", false, "convert markdown to standalone HTML")
	flags.BoolVar(&convertMode, "convert", false, "convert markdown to standalone HTML")
	flags.StringVar(&outputPath, "o", "", "output filename for conversion mode")
	flags.StringVar(&outputPath, "output", "", "output filename for conversion mode")
	flags.BoolVar(&showVersion, "v", false, "print version")
	flags.BoolVar(&showVersion, "version", false, "print version")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage: mdview [flags] [path]")
		fmt.Fprintln(flags.Output())
		fmt.Fprintln(flags.Output(), "Flags:")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return err
	}

	buildVersion := strings.TrimSpace(version)
	if showVersion {
		fmt.Println(buildVersion)
		return nil
	}

	targetPath := "."
	if flags.NArg() > 0 {
		targetPath = flags.Arg(0)
	}

	absTargetPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(absTargetPath)
	if err != nil {
		return fmt.Errorf("stat path: %w", err)
	}

	mdRenderer := renderer.New()
	if convertMode {
		if info.IsDir() {
			return errors.New("conversion mode requires a markdown file, not a directory")
		}

		content, err := os.ReadFile(absTargetPath)
		if err != nil {
			return fmt.Errorf("read markdown file: %w", err)
		}

		rendered, err := mdRenderer.Render(content, renderer.Options{
			Title: filepath.Base(absTargetPath),
		})
		if err != nil {
			return fmt.Errorf("render markdown: %w", err)
		}

		if outputPath == "" {
			outputPath = strings.TrimSuffix(absTargetPath, filepath.Ext(absTargetPath)) + ".html"
		} else if !filepath.IsAbs(outputPath) {
			outputPath, err = filepath.Abs(outputPath)
			if err != nil {
				return fmt.Errorf("resolve output path: %w", err)
			}
		}

		if err := os.WriteFile(outputPath, rendered, 0o644); err != nil {
			return fmt.Errorf("write HTML output: %w", err)
		}

		fmt.Println(outputPath)
		return nil
	}

	app, err := server.New(server.Config{
		Path:     absTargetPath,
		Port:     port,
		Host:     host,
		Version:  buildVersion,
		Renderer: mdRenderer,
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("mdview %s listening on http://%s\n", buildVersion, net.JoinHostPort(host, fmt.Sprintf("%d", port)))
	return app.Run(ctx)
}
