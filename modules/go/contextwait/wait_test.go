package contextwait

import (
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"testing"
	"testing/synctest"
	"time"
)

func TestEvalCases(t *testing.T) {
	data, err := os.ReadFile("../../../evals/modules/contextwait.json")
	if err != nil {
		t.Fatal(err)
	}
	var suite struct {
		Module string `json:"module"`
		Cases  []struct {
			Name     string `json:"name"`
			Context  string `json:"context"`
			Duration string `json:"duration"`
			Want     string `json:"want"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &suite, json.RejectUnknownMembers(true)); err != nil {
		t.Fatal(err)
	}
	if suite.Module != "go-contextwait" || len(suite.Cases) == 0 {
		t.Fatal("eval must name this module and contain cases")
	}
	names := map[string]bool{}
	for _, tc := range suite.Cases {
		if tc.Name == "" || names[tc.Name] {
			t.Fatalf("empty or duplicate case name: %q", tc.Name)
		}
		names[tc.Name] = true
		t.Run(tc.Name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				duration, err := time.ParseDuration(tc.Duration)
				if err != nil {
					t.Fatal(err)
				}
				var ctx context.Context
				switch tc.Context {
				case "background":
					ctx = context.Background()
				case "canceled":
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(context.Background())
					cancel()
				case "expired":
					var cancel context.CancelFunc
					ctx, cancel = context.WithDeadline(context.Background(), time.Unix(0, 0))
					defer cancel()
				case "nil":
				default:
					t.Fatalf("unsupported context: %q", tc.Context)
				}
				want, ok := map[string]error{
					"nil": nil, "canceled": context.Canceled,
					"deadline_exceeded": context.DeadlineExceeded, "nil_context": ErrNilContext,
				}[tc.Want]
				if !ok {
					t.Fatalf("unsupported expected result: %q", tc.Want)
				}
				start := time.Now()
				if got := Wait(ctx, duration); !errors.Is(got, want) {
					t.Fatalf("Wait returned %v, want %v", got, want)
				}
				var elapsed time.Duration
				if tc.Context == "background" && duration > 0 {
					elapsed = duration
				}
				if got := time.Since(start); got != elapsed {
					t.Fatalf("elapsed %v, want %v", got, elapsed)
				}
			})
		})
	}
}

func TestCancelDuringWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		result := make(chan error, 1)
		go func() { result <- Wait(ctx, time.Hour) }()
		// 待機を開始させてから中止し、実時間やスケジューラーの速度に依存しない。
		synctest.Wait()
		select {
		case err := <-result:
			t.Fatalf("wait completed before cancellation: %v", err)
		default:
		}
		cancel()
		synctest.Wait()
		select {
		case err := <-result:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("got %v, want cancellation", err)
			}
		default:
			t.Fatal("wait did not respond to cancellation")
		}
	})
}

func TestDeadlineDuringWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()
		start := time.Now()
		if err := Wait(ctx, time.Hour); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("got %v, want deadline exceeded", err)
		}
		if elapsed := time.Since(start); elapsed != time.Millisecond {
			t.Fatalf("elapsed %v, want context deadline after 1ms", elapsed)
		}
	})
}

func TestCancellationCauseUsesErr(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errors.New("upstream failed"))
	if err := Wait(ctx, time.Hour); err != context.Canceled {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestConcurrentWaiters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const count = 64
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		results := make(chan error, count)
		for range count {
			go func() { results <- Wait(ctx, time.Hour) }()
		}
		synctest.Wait()
		cancel()
		synctest.Wait()
		for range count {
			select {
			case err := <-results:
				if !errors.Is(err, context.Canceled) {
					t.Errorf("got %v, want canceled", err)
				}
			default:
				t.Fatal("concurrent waiters did not terminate")
			}
		}
		if err := Wait(context.Background(), time.Millisecond); err != nil {
			t.Fatalf("cancellation leaked between calls: %v", err)
		}
	})
}
