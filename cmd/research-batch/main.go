// Command research-batch runs isolated, bounded OpenCode research workers.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/shiroemons/ai-engineering-kit/internal/research"
)

func main() {
	root := flag.String("root", ".", "clean repository root")
	state := flag.String("state-dir", "", "persistent cooldown and log directory")
	progressPath := flag.String("progress-file", "", "optional JSON progress file")
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
	var report research.ProgressReporter
	if *progressPath != "" {
		var mu sync.Mutex
		report = func(progress research.Progress) error {
			mu.Lock()
			defer mu.Unlock()
			data, err := json.Marshal(progress)
			if err != nil {
				return err
			}
			temporary := *progressPath + ".tmp"
			if err := os.WriteFile(temporary, append(data, '\n'), 0600); err != nil {
				removeErr := os.Remove(temporary)
				if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
					return errors.Join(fmt.Errorf("write temporary progress file: %w", err), fmt.Errorf("remove temporary progress file: %w", removeErr))
				}
				return fmt.Errorf("write temporary progress file: %w", err)
			}
			if err := os.Rename(temporary, *progressPath); err != nil {
				removeErr := os.Remove(temporary)
				if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
					return errors.Join(fmt.Errorf("publish progress file: %w", err), fmt.Errorf("remove temporary progress file: %w", removeErr))
				}
				return fmt.Errorf("publish progress file: %w", err)
			}
			return nil
		}
	}
	if err := research.RunWithProgress(ctx, *root, *state, flag.Args(), *dry, os.Stdout, report); err != nil {
		fmt.Fprintln(os.Stderr, "research batch:", err)
		os.Exit(1)
	}
}
