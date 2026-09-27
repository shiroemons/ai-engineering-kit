package main

import (
	"testing"
	"time"

	"github.com/shiroemons/ai-engineering-kit/internal/research"
)

func TestAggregateProgressKeepsEachWorkerAndUsesAverage(t *testing.T) {
	updatedAt := time.Date(2026, time.September, 27, 0, 0, 0, 0, time.UTC)
	workers := map[string]research.Progress{
		"two": {Percent: 30, Domain: "two", Topic: "topic two", Phase: "資料確認", UpdatedAt: updatedAt},
		"one": {Percent: 70, Domain: "one", Topic: "topic one", Phase: "文書作成", UpdatedAt: updatedAt},
	}
	snapshot := aggregateProgress(workers, research.Progress{UpdatedAt: updatedAt}, 0)
	if snapshot.Percent != 50 || snapshot.DomainName != "2 workers" || snapshot.Topic != "topic one / topic two" {
		t.Fatalf("unexpected aggregate summary: %+v", snapshot)
	}
	if len(snapshot.Workers) != 2 || snapshot.Workers[0].Domain != "one" || snapshot.Workers[1].Domain != "two" {
		t.Fatalf("worker details are missing or unstable: %+v", snapshot.Workers)
	}
	newWorker := research.Progress{Percent: 3, Domain: "three", Topic: "topic three", Phase: "テーマ選定", UpdatedAt: updatedAt}
	workers["three"] = newWorker
	snapshot = aggregateProgress(workers, research.Progress{UpdatedAt: updatedAt}, snapshot.Percent)
	if snapshot.Percent != 50 {
		t.Fatalf("adding a newly started worker moved the displayed progress backward: %d", snapshot.Percent)
	}
}

func TestAggregateProgressUsesUnscopedStateBeforeWorkersStart(t *testing.T) {
	updatedAt := time.Date(2026, time.September, 27, 0, 0, 0, 0, time.UTC)
	latest := research.Progress{Percent: 1, Topic: "調査テーマ", Phase: "既存知識を確認中", UpdatedAt: updatedAt}
	snapshot := aggregateProgress(nil, latest, 0)
	if snapshot.Percent != latest.Percent || snapshot.Topic != latest.Topic || snapshot.Phase != latest.Phase || !snapshot.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("initial progress was not retained: %+v", snapshot)
	}
}
