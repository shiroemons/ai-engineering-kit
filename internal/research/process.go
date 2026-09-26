package research

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unicode"
)

var errRateLimit = errors.New("provider rate or quota limit; batch stopped")

// OpenCode can retry internally for a long time. Inspect error events as they
// arrive, and kill the entire standalone server group on cancellation.
type events struct {
	pending      []byte
	Topic        string
	Err          error
	Cancel       context.CancelCauseFunc
	StopBatch    context.CancelCauseFunc
	Domain       domain
	Report       ProgressReporter
	lastProgress string
	textPending  string
}

func (e *events) Write(p []byte) (int, error) {
	e.pending = append(e.pending, p...)
	for {
		i := bytes.IndexByte(e.pending, '\n')
		if i < 0 {
			break
		}
		e.line(e.pending[:i])
		e.pending = e.pending[i+1:]
	}
	if len(e.pending) > 4<<20 {
		e.Err = errors.New("OpenCode event exceeds 4 MiB")
		e.Cancel(e.Err)
		return 0, e.Err
	}
	return len(p), nil
}

func (e *events) line(line []byte) {
	var event struct {
		Type string `json:"type"`
		Part struct {
			Text string `json:"text"`
		} `json:"part"`
		Status struct {
			Type string `json:"type"`
		} `json:"status"`
	}
	if json.Unmarshal(line, &event) != nil {
		return
	}
	if event.Type == "error" || event.Type == "retry" || event.Status.Type == "retry" {
		lower := strings.ToLower(string(line))
		e.Err = errors.New("OpenCode reported a provider error")
		for _, term := range []string{"429", "rate limit", "rate_limit", "quota", "too many requests"} {
			if strings.Contains(lower, term) {
				e.Err = errRateLimit
				e.StopBatch(errRateLimit)
				break
			}
		}
		e.Cancel(e.Err)
	}
	if event.Type == "text" {
		e.text(event.Part.Text)
	}
}

func (e *events) text(text string) {
	combined := e.textPending + text
	lines := strings.Split(combined, "\n")
	for _, line := range lines[:len(lines)-1] {
		e.textLine(line)
	}
	lastLine := strings.TrimSpace(lines[len(lines)-1])
	if protocolPrefixCandidate(lastLine) && len(lastLine) <= 4096 {
		e.textPending = lines[len(lines)-1]
	} else {
		e.textPending = ""
	}
	// TOPIC and PROGRESS are short protocol lines. Parse them while they stream,
	// even if OpenCode has not emitted the trailing newline yet.
	trimmed := strings.TrimSpace(e.textPending)
	if strings.HasPrefix(trimmed, "TOPIC:") || strings.HasPrefix(trimmed, "PROGRESS:") {
		e.textLine(trimmed)
	}
}

func protocolPrefixCandidate(text string) bool {
	for _, prefix := range []string{"TOPIC:", "PROGRESS:"} {
		if strings.HasPrefix(prefix, text) || strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func (e *events) textLine(line string) {
	line = strings.TrimSpace(line)
	if topic, ok := strings.CutPrefix(line, "TOPIC:"); ok {
		topic = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, strings.TrimSpace(topic))
		runes := []rune(topic)
		if len(runes) > 120 {
			topic = string(runes[:120])
		}
		if topic != "" && topic != e.Topic {
			e.Topic = topic
			e.publish(10, "テーマを選定")
		}
	}
	marker, ok := strings.CutPrefix(line, "PROGRESS:")
	if !ok {
		return
	}
	marker = strings.TrimSpace(marker)
	if marker == e.lastProgress {
		return
	}
	percent, phase, ok := progressMilestone(marker)
	if !ok {
		return
	}
	e.lastProgress = marker
	e.publish(percent, phase)
}

func (e *events) publish(percent int, phase string) {
	if e.Report == nil || e.Err != nil {
		return
	}
	progress := Progress{Percent: percent, Domain: e.Domain.ID, DomainName: e.Domain.Name, Topic: e.Topic, Phase: phase}
	if err := e.Report(progress); err != nil {
		e.Err = fmt.Errorf("write research progress: %w", err)
		e.Cancel(e.Err)
		e.StopBatch(e.Err)
	}
}

func runOpenCode(ctx context.Context, root, model, prompt string, stopBatch context.CancelCauseFunc, d domain, report ProgressReporter) (string, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	e := &events{Cancel: cancel, StopBatch: stopBatch, Domain: d, Report: report}
	cmd := exec.CommandContext(ctx, "opencode", "run", "--standalone", "--format", "json", "--title", "Knowledge research", "--agent", "knowledge-researcher", "--model", model, prompt)
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	// Same writer means os/exec serializes stdout and stderr copies.
	cmd.Stdout, cmd.Stderr = e, e
	err := cmd.Run()
	if len(e.pending) > 0 {
		e.line(e.pending)
	}
	if e.textPending != "" {
		e.textLine(e.textPending)
	}
	if e.Err != nil {
		return "", e.Err
	}
	if ctx.Err() != nil {
		return "", context.Cause(ctx)
	}
	if err != nil {
		return "", fmt.Errorf("OpenCode process: %w", err)
	}
	if e.Topic == "" {
		return "", errors.New("OpenCode returned no TOPIC")
	}
	return e.Topic, nil
}

func progressMilestone(marker string) (int, string, bool) {
	switch marker {
	case "topic-selected":
		return 10, "テーマを選定", true
	case "sources-verified":
		return 30, "公式一次資料を照合", true
	case "knowledge-written":
		return 55, "knowledge 文書を作成", true
	case "eval-written":
		return 70, "検索 eval を作成", true
	case "eval-search-verified":
		return 85, "検索 eval の検索結果を確認", true
	case "ready-to-validate":
		return 90, "成果物の最終検証を開始", true
	default:
		return 0, "", false
	}
}

func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	if len(args) > 0 && args[0] == "diff" {
		// Machine-readable patches must not inherit an interactive diff tool,
		// custom path prefixes, text conversion, or color from user Git config.
		args = append([]string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--src-prefix=a/", "--dst-prefix=b/"}, args[1:]...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(diagnostic.String()))
	}
	return data, nil
}
