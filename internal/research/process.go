package research

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
)

var errRateLimit = errors.New("provider rate or quota limit; batch stopped")

// OpenCode can retry internally for a long time. Inspect error events as they
// arrive, and kill the entire standalone server group on cancellation.
type events struct {
	pending          []byte
	textOutput       bytes.Buffer
	Topic            string
	TopicSelected    bool
	TopicFinal       bool
	SourcesVerified  bool
	KnowledgeWritten bool
	Err              error
	Cancel           context.CancelCauseFunc
	StopBatch        context.CancelCauseFunc
	Domain           domain
	Report           ProgressReporter
	lastProgress     string
	textPending      string
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
		e.textPart(event.Part.Text)
	}
}

// OpenCode emits one completed text part per text event. Keep those boundaries
// when assembling a phase response so a progress/commentary part without a
// trailing newline cannot join the next part's first protocol line.
func (e *events) textPart(text string) {
	if text == "" {
		return
	}
	output := e.textOutput.Bytes()
	if len(output) > 0 && output[len(output)-1] != '\n' {
		e.text("\n")
	}
	e.text(text)
}

func (e *events) text(text string) {
	if e.textOutput.Len()+len(text) > 32<<10 {
		e.Err = errors.New("OpenCode text output exceeds 32 KiB")
		e.Cancel(e.Err)
		e.StopBatch(e.Err)
		return
	}
	_, _ = e.textOutput.WriteString(text)
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
	// Topic and progress markers are short protocol lines. Parse them while
	// they stream, even if OpenCode has not emitted the trailing newline yet.
	trimmed := strings.TrimSpace(e.textPending)
	if protocolPrefixCandidate(trimmed) {
		e.textLine(trimmed)
	}
}

func protocolPrefixCandidate(text string) bool {
	for _, prefix := range []string{"TOPIC_SELECTED:", "TOPIC:", "PROGRESS:"} {
		if strings.HasPrefix(prefix, text) || strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func (e *events) textLine(line string) {
	line = strings.TrimSpace(line)
	if topic, ok := strings.CutPrefix(line, "TOPIC_SELECTED:"); ok {
		e.setTopic(topic, false)
		return
	}
	if topic, ok := strings.CutPrefix(line, "TOPIC:"); ok {
		e.setTopic(topic, true)
		return
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
	switch marker {
	case "sources-verified":
		e.SourcesVerified = true
	case "knowledge-written":
		e.KnowledgeWritten = true
	}
	e.publish(percent, phase)
}

func (e *events) setTopic(topic string, final bool) {
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
	if topic == "" || (e.TopicFinal && !final) {
		return
	}
	e.Topic = topic
	if final {
		e.TopicFinal = true
		return
	}
	e.TopicSelected = true
	e.publish(10, "テーマを選定")
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

func runOpenCodeEvalWriting(ctx context.Context, root, model, prompt string, stopBatch context.CancelCauseFunc, d domain, report ProgressReporter) error {
	_, err := runOpenCodePhase(ctx, root, model, prompt, stopBatch, d, report, completeEvalWriting)
	return err
}

func runOpenCodeTopicSelection(ctx context.Context, root, model, prompt string, stopBatch context.CancelCauseFunc, d domain, report ProgressReporter) (string, error) {
	result, err := runOpenCodePhase(ctx, root, model, prompt, stopBatch, d, report, completeTopicSelection)
	return result.Topic, err
}

func runOpenCodeTopicOverlapCheck(ctx context.Context, root, model, prompt string, stopBatch context.CancelCauseFunc, d domain, report ProgressReporter) (bool, error) {
	result, err := runOpenCodePhase(ctx, root, model, prompt, stopBatch, d, report, completeTopicOverlapCheck)
	if err != nil {
		return false, err
	}
	var decision string
	for line := range strings.SplitSeq(result.Text, "\n") {
		line = strings.TrimSpace(line)
		value, ok := strings.CutPrefix(line, "TOPIC_OVERLAP:")
		if !ok {
			continue
		}
		if decision != "" {
			return false, errors.New("OpenCode returned multiple topic-overlap decisions")
		}
		decision = strings.TrimSpace(value)
	}
	switch decision {
	case "DISTINCT":
		return false, nil
	case "DUPLICATE":
		return true, nil
	default:
		return false, errors.New("OpenCode returned no valid topic-overlap decision")
	}
}

func runOpenCodeSourceVerification(ctx context.Context, root, model, prompt string, stopBatch context.CancelCauseFunc, d domain, report ProgressReporter) (string, error) {
	return runOpenCodeSourceVerificationStarted(ctx, root, model, prompt, stopBatch, d, report, nil)
}

func runOpenCodeSourceVerificationStarted(ctx context.Context, root, model, prompt string, stopBatch context.CancelCauseFunc, d domain, report ProgressReporter, started func()) (string, error) {
	result, err := runOpenCodePhaseStarted(ctx, root, model, prompt, stopBatch, d, report, completeSourceVerification, started)
	return result.Text, err
}

func runOpenCodeKnowledgeWriting(ctx context.Context, root, model, prompt string, stopBatch context.CancelCauseFunc, d domain, report ProgressReporter) error {
	_, err := runOpenCodePhase(ctx, root, model, prompt, stopBatch, d, report, completeKnowledgeWriting)
	return err
}

type phaseCompletion uint8

const (
	completeTopicSelection phaseCompletion = iota
	completeTopicOverlapCheck
	completeSourceVerification
	completeKnowledgeWriting
	completeEvalWriting
)

type modelOutput struct {
	Topic string
	Text  string
}

type openCodeProcessReporterKey struct{}

type openCodeProcessReporter func(int) error

func processStartIdentity(pid int) (string, error) {
	output, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "lstart=").Output()
	if err != nil {
		return "", err
	}
	identity := strings.TrimSpace(string(output))
	if identity == "" {
		return "", errors.New("process start identity is empty")
	}
	return identity, nil
}

func processIdentityAlive(pid int, expectedStart string) bool {
	if pid <= 1 {
		return false
	}
	if expectedStart != "" {
		actualStart, err := processStartIdentity(pid)
		return err == nil && actualStart == expectedStart
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func processGroupIsAlive(pid int) bool {
	if pid <= 1 {
		return false
	}
	err := syscall.Kill(-pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func runOpenCodePhase(ctx context.Context, root, model, prompt string, stopBatch context.CancelCauseFunc, d domain, report ProgressReporter, completion phaseCompletion) (modelOutput, error) {
	return runOpenCodePhaseStarted(ctx, root, model, prompt, stopBatch, d, report, completion, nil)
}

func runOpenCodePhaseStarted(ctx context.Context, root, model, prompt string, stopBatch context.CancelCauseFunc, d domain, report ProgressReporter, completion phaseCompletion, started func()) (modelOutput, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	e := &events{Cancel: cancel, StopBatch: stopBatch, Domain: d, Report: report}
	gateReader, gateWriter, err := os.Pipe()
	if err != nil {
		return modelOutput{}, fmt.Errorf("create OpenCode start gate: %w", err)
	}
	args := []string{"opencode", "run", "--standalone", "--format", "json", "--title", "Knowledge research", "--agent", "knowledge-researcher", "--model", model, prompt}
	commandArgs := append([]string{"-c", `IFS= read -r _ <&3 || exit 125; exec "$@"`, "opencode-start-gate"}, args...)
	cmd := exec.CommandContext(ctx, "/bin/sh", commandArgs...)
	cmd.ExtraFiles = []*os.File{gateReader}
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil {
			return err
		}
		for attempt := 0; attempt < 30; attempt++ {
			if !processGroupIsAlive(cmd.Process.Pid) {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		for attempt := 0; attempt < 20; attempt++ {
			if !processGroupIsAlive(cmd.Process.Pid) {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		return errors.New("OpenCode process group remained alive after SIGKILL")
	}
	cmd.WaitDelay = 2 * time.Second
	// Same writer means os/exec serializes stdout and stderr copies.
	cmd.Stdout, cmd.Stderr = e, e
	err = cmd.Start()
	_ = gateReader.Close()
	if err == nil {
		var reportErr error
		if reporter, ok := ctx.Value(openCodeProcessReporterKey{}).(openCodeProcessReporter); ok {
			if reportErr = reporter(cmd.Process.Pid); reportErr != nil {
				cancel(reportErr)
			}
		}
		if reportErr == nil && ctx.Err() == nil {
			_, reportErr = io.WriteString(gateWriter, "start\n")
			if reportErr != nil {
				cancel(fmt.Errorf("release OpenCode start gate: %w", reportErr))
			}
		}
		if closeErr := gateWriter.Close(); closeErr != nil {
			reportErr = errors.Join(reportErr, closeErr)
			cancel(closeErr)
		}
		if reportErr == nil && ctx.Err() == nil && started != nil {
			started()
		}
		err = cmd.Wait()
		if reportErr != nil {
			err = errors.Join(err, reportErr)
		}
		if ctx.Err() != nil {
			err = errors.Join(err, context.Cause(ctx))
		}
		if processGroupIsAlive(cmd.Process.Pid) {
			err = errors.Join(err, errors.New("OpenCode process group remained after command exit"))
		} else if reporter, ok := ctx.Value(openCodeProcessReporterKey{}).(openCodeProcessReporter); ok {
			if reportErr := reporter(0); reportErr != nil {
				err = errors.Join(err, reportErr)
			}
		}
	} else {
		_ = gateWriter.Close()
	}
	if len(e.pending) > 0 {
		e.line(e.pending)
	}
	if e.textPending != "" {
		e.textLine(e.textPending)
	}
	if e.Err != nil {
		return modelOutput{}, e.Err
	}
	if ctx.Err() != nil {
		return modelOutput{}, context.Cause(ctx)
	}
	if err != nil {
		return modelOutput{}, fmt.Errorf("OpenCode process: %w", err)
	}
	switch completion {
	case completeTopicSelection:
		if e.TopicFinal {
			return modelOutput{}, errors.New("OpenCode emitted a final TOPIC marker during topic selection")
		}
		if !e.TopicSelected {
			return modelOutput{}, errors.New("OpenCode returned no TOPIC_SELECTED marker")
		}
	case completeTopicOverlapCheck:
		if e.TopicSelected || e.TopicFinal {
			return modelOutput{}, errors.New("OpenCode emitted a topic completion marker during overlap checking")
		}
		return modelOutput{Text: e.textOutput.String()}, nil
	case completeSourceVerification:
		if e.TopicSelected || e.TopicFinal {
			return modelOutput{}, errors.New("OpenCode emitted a topic completion marker during source verification")
		}
		if !e.SourcesVerified {
			return modelOutput{}, errors.New("OpenCode returned without the sources-verified milestone")
		}
	case completeKnowledgeWriting:
		if e.TopicSelected || e.TopicFinal {
			return modelOutput{}, errors.New("OpenCode emitted a topic completion marker during knowledge writing")
		}
		if !e.KnowledgeWritten {
			return modelOutput{}, errors.New("OpenCode returned without the knowledge-written milestone")
		}
	case completeEvalWriting:
		// Artifact and eval checks in the runner decide whether this phase completed.
		return modelOutput{Text: e.textOutput.String()}, nil
	default:
		return modelOutput{}, errors.New("unknown OpenCode completion phase")
	}
	return modelOutput{Topic: e.Topic, Text: e.textOutput.String()}, nil
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
