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
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/shiroemons/ai-engineering-kit/internal/research"
)

func main() {
	root := flag.String("root", ".", "clean repository root")
	state := flag.String("state-dir", "", "persistent cooldown and log directory")
	runDir := flag.String("run-dir", "", "persistent state directory for this research run")
	runID := flag.String("run-id", "", "stable ID for this research run")
	resume := flag.Bool("resume", false, "resume workers from the saved run state")
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
		workers := map[string]research.Progress{}
		var latest research.Progress
		highestPercent := 0
		report = func(progress research.Progress) error {
			mu.Lock()
			defer mu.Unlock()
			latest.UpdatedAt = progress.UpdatedAt
			if progress.Domain == "" {
				latest = progress
			} else {
				workers[progress.Domain] = progress
			}
			snapshot := aggregateProgress(workers, latest, highestPercent)
			if snapshot.Percent > highestPercent {
				highestPercent = snapshot.Percent
			}
			data, err := json.Marshal(snapshot)
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
	if err := research.RunWithRecovery(ctx, *root, *state, flag.Args(), *dry, os.Stdout, report, *runDir, *runID, *resume); err != nil {
		fmt.Fprintln(os.Stderr, "research batch:", err)
		if errors.Is(err, research.ErrRateLimit) {
			os.Exit(6)
		}
		if errors.Is(err, research.ErrInvalidBaselineEvals) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

type progressSnapshot struct {
	Percent    int                 `json:"percent"`
	DomainName string              `json:"domain_name,omitempty"`
	Topic      string              `json:"topic,omitempty"`
	Phase      string              `json:"phase"`
	UpdatedAt  time.Time           `json:"updated_at"`
	Workers    []research.Progress `json:"workers,omitempty"`
}

func aggregateProgress(workers map[string]research.Progress, latest research.Progress, highestPercent int) progressSnapshot {
	if len(workers) == 0 {
		return progressSnapshot{
			Percent: max(latest.Percent, highestPercent), DomainName: latest.DomainName, Topic: latest.Topic,
			Phase: latest.Phase, UpdatedAt: latest.UpdatedAt,
		}
	}
	domains := make([]string, 0, len(workers))
	for domain := range workers {
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	snapshot := progressSnapshot{
		DomainName: fmt.Sprintf("%d workers", len(domains)),
		Phase:      "複数ワーカーを並列実行中",
		UpdatedAt:  latest.UpdatedAt,
		Workers:    make([]research.Progress, 0, len(domains)),
	}
	topics := make([]string, 0, len(domains))
	seenTopics := map[string]bool{}
	for _, domain := range domains {
		progress := workers[domain]
		snapshot.Percent += progress.Percent
		snapshot.Workers = append(snapshot.Workers, progress)
		if progress.Topic != "" && progress.Topic != "テーマ候補と一次資料を確認中" && !seenTopics[progress.Topic] {
			topics = append(topics, progress.Topic)
			seenTopics[progress.Topic] = true
		}
	}
	snapshot.Percent /= len(domains)
	snapshot.Percent = max(snapshot.Percent, highestPercent)
	if len(topics) == 0 {
		snapshot.Topic = "テーマ選定中"
	} else {
		snapshot.Topic = strings.Join(topics, " / ")
	}
	return snapshot
}
