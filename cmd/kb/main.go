// Command kb はローカルの技術ナレッジを検証・検索する。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shiroemons/ai-engineering-kit/internal/kb"
)

const usage = `Usage: kb [options] COMMAND [QUERY] [options]

Commands:
  validate      Validate configuration, sources, and document metadata
  freshness     List expired or explicitly stale documents
  index         Rebuild generated RAG documents, metadata, and search index
  search QUERY  Search knowledge, patterns, and reusable modules
  stats         Show document, stale, and source counts
  help          Show this help

Options (accepted before or after the command and query):
  --root PATH          Repository root (default: discover from current directory)
  --json               Output JSON only on stdout
  --as-of YYYY-MM-DD   Evaluate freshness on this UTC date (default: today)
  --limit N            Maximum search results, a positive integer (default: 20)
  -h, --help           Show this help

Use -- to treat remaining words as positional arguments.
Example: kb search "context cancellation" --json
`

type options struct {
	command  string
	query    string
	root     string
	json     bool
	now      time.Time
	limit    int
	limitSet bool
	help     bool
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, time.Now()))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, now time.Time) int {
	opts, err := parseArgs(args, now)
	if err != nil {
		return reportError(stderr, 2, err)
	}
	if opts.help {
		if _, err := io.WriteString(stdout, usage); err != nil {
			return reportError(stderr, 1, fmt.Errorf("write output: %w", err))
		}
		return 0
	}
	if opts.root == "" {
		opts.root, err = discoverRoot()
		if err != nil {
			return reportError(stderr, 1, err)
		}
	}
	repo, err := kb.Load(opts.root, opts.now)
	if err == nil {
		err = execute(ctx, repo, opts, stdout)
	}
	if err != nil {
		return reportError(stderr, 1, err)
	}
	return 0
}

func reportError(stderr io.Writer, status int, err error) int {
	message := fmt.Sprintf("kb: %v\n", err)
	if status == 2 {
		message += "Run 'kb help' for usage.\n"
	}
	if _, writeErr := io.WriteString(stderr, message); writeErr != nil {
		// 診断を書き込めない場合も、出力障害として終了コードに反映する。
		return 1
	}
	return status
}

func parseArgs(args []string, now time.Time) (options, error) {
	opts := options{now: now, limit: 20}
	var positional []string
	literal := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if literal {
			positional = append(positional, arg)
			continue
		}
		if arg == "--" {
			literal = true
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		name, value, hasValue := strings.Cut(arg, "=")
		switch name {
		case "-h", "--help", "--json":
			if hasValue {
				return opts, fmt.Errorf("%s does not take a value", name)
			}
			if name == "--json" {
				opts.json = true
			} else {
				opts.help = true
			}
		case "--root", "--as-of", "--limit":
			if !hasValue {
				if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
					return opts, fmt.Errorf("%s requires a value", name)
				}
				i++
				value = args[i]
			}
			if strings.TrimSpace(value) == "" {
				return opts, fmt.Errorf("%s requires a non-empty value", name)
			}
			switch name {
			case "--root":
				opts.root = value
			case "--as-of":
				date, err := time.Parse(time.DateOnly, value)
				if err != nil {
					return opts, fmt.Errorf("invalid --as-of %q: use YYYY-MM-DD", value)
				}
				opts.now = date
			case "--limit":
				limit, err := strconv.Atoi(value)
				if err != nil || limit <= 0 {
					return opts, fmt.Errorf("--limit must be a positive integer")
				}
				opts.limit, opts.limitSet = limit, true
			}
		default:
			return opts, fmt.Errorf("unknown option %q", name)
		}
	}
	if len(positional) == 0 {
		if opts.help || len(args) == 0 {
			opts.help = true
			return opts, nil
		}
		return opts, fmt.Errorf("a command is required")
	}
	opts.command = positional[0]
	switch opts.command {
	case "help":
		opts.help = true
	case "validate", "freshness", "index", "stats", "search":
	default:
		return opts, fmt.Errorf("unknown command %q", opts.command)
	}
	if opts.help {
		return opts, nil
	}
	if opts.command == "search" {
		opts.query = strings.TrimSpace(strings.Join(positional[1:], " "))
		if opts.query == "" {
			return opts, fmt.Errorf("search requires a non-empty query")
		}
	} else {
		if len(positional) != 1 {
			return opts, fmt.Errorf("%s does not accept positional arguments", opts.command)
		}
		if opts.limitSet {
			return opts, fmt.Errorf("--limit is only supported by search")
		}
	}
	return opts, nil
}

func discoverRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		if isRepositoryRoot(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repository root not found; specify --root PATH")
		}
		dir = parent
	}
}

func isRepositoryRoot(dir string) bool {
	for _, path := range []string{"go.mod", "config/freshness.json", "config/sources.json"} {
		info, err := os.Stat(filepath.Join(dir, path))
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

func execute(ctx context.Context, repo *kb.Repository, opts options, stdout io.Writer) error {
	switch opts.command {
	case "validate":
		if err := repo.Validate(); err != nil {
			return err
		}
		return writeMessage(stdout, opts.json, "validation passed")
	case "index":
		if err := repo.Index(); err != nil {
			return err
		}
		return writeMessage(stdout, opts.json, "index rebuilt")
	case "stats":
		return writeJSON(stdout, repo.Stats())
	case "freshness":
		return writeResults(stdout, repo.Freshness(), opts.json)
	case "search":
		results, err := repo.Search(ctx, opts.query, opts.limit)
		if err != nil {
			return err
		}
		return writeResults(stdout, results, opts.json)
	default:
		return fmt.Errorf("unsupported command %q", opts.command)
	}
}

func writeMessage(w io.Writer, asJSON bool, message string) error {
	if asJSON {
		return writeJSON(w, map[string]string{"status": "ok", "message": message})
	}
	_, err := fmt.Fprintln(w, message)
	return err
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func writeResults(w io.Writer, results []kb.Result, asJSON bool) error {
	if results == nil {
		results = []kb.Result{}
	}
	if asJSON {
		return writeJSON(w, results)
	}
	if len(results) == 0 {
		_, err := fmt.Fprintln(w, "No results.")
		return err
	}
	// 検索バックエンドにメタデータが追加されても、端末上で全項目を確認できるようにする。
	for _, result := range results {
		if err := writeJSON(w, result); err != nil {
			return err
		}
	}
	return nil
}
