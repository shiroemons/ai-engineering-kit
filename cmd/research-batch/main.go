// Command research-batch runs isolated, bounded OpenCode research workers.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shiroemons/ai-engineering-kit/internal/research"
)

func main() {
	root := flag.String("root", ".", "clean repository root")
	state := flag.String("state-dir", "", "persistent cooldown and log directory")
	dry := flag.Bool("dry-run", false, "select topics without accepting edits")
	deadline := flag.Int64("deadline", 0, "optional Unix timestamp for continuous execution")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *deadline != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, time.Unix(*deadline, 0))
		defer cancel()
	}
	if err := research.Run(ctx, *root, *state, flag.Args(), *dry, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "research batch:", err)
		os.Exit(1)
	}
}
