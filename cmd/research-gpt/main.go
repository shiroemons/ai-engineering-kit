// Command research-gpt plans and validates bounded GPT research artifacts.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/shiroemons/ai-engineering-kit/internal/research"
)

const usage = `Usage:
  research-gpt plan [--topics 3] [--root .]
  research-gpt validate --base <40-character SHA> [--topics 3] [--root .]

Commands:
  plan       Select bounded research topics and describe required artifacts
  validate   Validate research artifacts against a baseline Git commit
  help       Show this help

Options (must follow the command):
  --topics N   Number of topics, from 1 to 3 (default: 3)
  --root PATH  Repository root (default: .)
  --base SHA   Full 40-character hexadecimal baseline commit (validate only)
  -h, --help   Show this help

Successful plan and validate results are JSON on stdout.
Help and diagnostics are written to stderr.
`

type options struct {
	command string
	root    string
	base    string
	topics  int
	help    bool
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	status := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(status)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	opts, err := parseArgs(args)
	if err != nil {
		return reportError(stderr, 2, err)
	}
	if opts.help {
		if _, err := io.WriteString(stderr, usage); err != nil {
			return 1
		}
		return 0
	}

	var result any
	switch opts.command {
	case "plan":
		result, err = research.GPTPlan(ctx, opts.root, opts.topics)
	case "validate":
		result, err = research.ValidateGPT(ctx, opts.root, opts.base, opts.topics)
	}
	if err != nil {
		return reportError(stderr, 1, err)
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		return reportError(stderr, 1, fmt.Errorf("write JSON output: %w", err))
	}
	return 0
}

func reportError(stderr io.Writer, status int, err error) int {
	message := fmt.Sprintf("research-gpt: %v\n", err)
	if status == 2 {
		message += "Run 'research-gpt help' for usage.\n"
	}
	if _, err := io.WriteString(stderr, message); err != nil {
		return 1
	}
	return status
}

func parseArgs(args []string) (options, error) {
	opts := options{root: ".", topics: 3}
	if len(args) == 0 {
		return opts, fmt.Errorf("a command is required (plan or validate)")
	}
	opts.command = args[0]
	switch opts.command {
	case "help", "-h", "--help":
		if len(args) != 1 {
			return opts, fmt.Errorf("unexpected arguments after help")
		}
		opts.help = true
		return opts, nil
	case "plan", "validate":
	default:
		return opts, fmt.Errorf("unknown command %q (options must follow the command)", opts.command)
	}

	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				return opts, fmt.Errorf("unexpected positional argument %q", args[i+1])
			}
			break
		}
		if !strings.HasPrefix(arg, "-") {
			return opts, fmt.Errorf("unexpected positional argument %q", arg)
		}
		name, value, hasValue := strings.Cut(arg, "=")
		switch name {
		case "-h", "--help":
			if hasValue {
				return opts, fmt.Errorf("%s does not take a value", name)
			}
			opts.help = true
			continue
		case "--topics", "--root":
		case "--base":
			if opts.command != "validate" {
				return opts, fmt.Errorf("--base is only valid for validate")
			}
		default:
			return opts, fmt.Errorf("unknown option %q", name)
		}
		if !hasValue {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") || args[i+1] == "-h" {
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
		case "--base":
			opts.base = value
		case "--topics":
			topics, err := strconv.Atoi(value)
			if err != nil || topics < 1 || topics > 3 {
				return opts, fmt.Errorf("--topics must be an integer from 1 to 3")
			}
			opts.topics = topics
		}
	}
	if opts.command == "validate" && !opts.help {
		if opts.base == "" {
			return opts, fmt.Errorf("validate requires --base <40-character SHA>")
		}
		if len(opts.base) != 40 {
			return opts, fmt.Errorf("--base must be a full 40-character hexadecimal commit SHA")
		}
		if _, err := hex.DecodeString(opts.base); err != nil {
			return opts, fmt.Errorf("--base must be a full 40-character hexadecimal commit SHA")
		}
	}
	return opts, nil
}
