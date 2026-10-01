package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

const testBase = "0123456789abcdef0123456789abcdef01234567"

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want options
	}{
		{"plan defaults", []string{"plan"}, options{command: "plan", root: ".", topics: 3}},
		{"plan options", []string{"plan", "--topics", "1", "--root", "/tmp/repository with spaces"}, options{command: "plan", root: "/tmp/repository with spaces", topics: 1}},
		{"equals options", []string{"plan", "--topics=2", "--root=elsewhere"}, options{command: "plan", root: "elsewhere", topics: 2}},
		{"validate defaults", []string{"validate", "--base", testBase}, options{command: "validate", root: ".", topics: 3, base: testBase}},
		{"validate options", []string{"validate", "--root=repo", "--base=" + testBase, "--topics=1"}, options{command: "validate", root: "repo", topics: 1, base: testBase}},
		{"uppercase base", []string{"validate", "--base", strings.ToUpper(testBase)}, options{command: "validate", root: ".", topics: 3, base: strings.ToUpper(testBase)}},
		{"trailing separator", []string{"plan", "--"}, options{command: "plan", root: ".", topics: 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args)
			if err != nil || got != tt.want {
				t.Fatalf("parseArgs(%q) = %+v, %v; want %+v", tt.args, got, err, tt.want)
			}
		})
	}
}

func TestUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing command", nil, "command is required"},
		{"unknown command", []string{"publish"}, "unknown command"},
		{"options before command", []string{"--topics", "2", "plan"}, "options must follow"},
		{"unknown plan option", []string{"plan", "--unknown"}, "unknown option"},
		{"unknown validate option", []string{"validate", "--base", testBase, "--unknown"}, "unknown option"},
		{"unexpected positional", []string{"plan", "extra"}, "unexpected positional"},
		{"extra command", []string{"plan", "validate"}, "unexpected positional"},
		{"extra after separator", []string{"plan", "--", "extra"}, "unexpected positional"},
		{"zero topics", []string{"plan", "--topics=0"}, "integer from 1 to 3"},
		{"negative topics", []string{"plan", "--topics", "-1"}, "integer from 1 to 3"},
		{"too many topics", []string{"plan", "--topics=4"}, "integer from 1 to 3"},
		{"noninteger topics", []string{"plan", "--topics=1.5"}, "integer from 1 to 3"},
		{"nonnumeric topics", []string{"plan", "--topics=three"}, "integer from 1 to 3"},
		{"missing topics", []string{"plan", "--topics"}, "requires a value"},
		{"empty topics", []string{"plan", "--topics="}, "non-empty value"},
		{"missing root", []string{"plan", "--root"}, "requires a value"},
		{"empty root", []string{"plan", "--root="}, "non-empty value"},
		{"blank root", []string{"plan", "--root", " "}, "non-empty value"},
		{"root followed by option", []string{"plan", "--root", "--topics=2"}, "requires a value"},
		{"base on plan", []string{"plan", "--base", testBase}, "only valid for validate"},
		{"missing base", []string{"validate"}, "requires --base"},
		{"missing base value", []string{"validate", "--base"}, "requires a value"},
		{"empty base", []string{"validate", "--base="}, "non-empty value"},
		{"symbolic base", []string{"validate", "--base=HEAD"}, "40-character hexadecimal"},
		{"short base", []string{"validate", "--base=" + testBase[:39]}, "40-character hexadecimal"},
		{"long base", []string{"validate", "--base=" + testBase + "0"}, "40-character hexadecimal"},
		{"nonhex base", []string{"validate", "--base=" + strings.Repeat("g", 40)}, "40-character hexadecimal"},
		{"validate topics invalid", []string{"validate", "--base", testBase, "--topics=4"}, "integer from 1 to 3"},
		{"help value", []string{"plan", "--help=true"}, "does not take a value"},
		{"extra after help", []string{"help", "plan"}, "unexpected arguments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			status := run(context.Background(), tt.args, &stdout, &stderr)
			if status != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("run(%q) status=%d stdout=%q stderr=%q; want status 2, empty stdout, and %q diagnostic", tt.args, status, stdout.String(), stderr.String(), tt.want)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}, {"plan", "-h"}, {"plan", "--help"}, {"validate", "--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			status := run(context.Background(), args, &stdout, &stderr)
			if status != 0 || stdout.Len() != 0 || stderr.String() != usage {
				t.Fatalf("help status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
			}
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("output unavailable")
}

func TestDiagnosticWriteFailures(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"unknown-command"}} {
		var stdout bytes.Buffer
		if status := run(context.Background(), args, &stdout, failingWriter{}); status != 1 {
			t.Fatalf("run(%q) status=%d; want output failure status 1", args, status)
		}
		if stdout.Len() != 0 {
			t.Fatalf("run(%q) unexpectedly wrote to stdout: %q", args, stdout.String())
		}
	}
}
