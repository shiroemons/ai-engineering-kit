package research

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"
	"unicode"
)

var errTopicAlreadyActive = errors.New("topic is already being researched")

const maxTopicLeaseAge = 48 * time.Hour

type activeTopic struct {
	Topic     string    `json:"topic"`
	Domain    string    `json:"domain"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

func normalizedTopic(topic string) string {
	var normalized strings.Builder
	separating := false
	for _, r := range strings.TrimSpace(topic) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			if separating && normalized.Len() > 0 {
				normalized.WriteByte('-')
			}
			normalized.WriteRune(unicode.ToLower(r))
			separating = false
		} else {
			separating = true
		}
	}
	return normalized.String()
}

func reserveTopic(state string, d domain, topic string) (string, error) {
	normalized := normalizedTopic(topic)
	if normalized == "" {
		return "", errors.New("selected topic has no letters or numbers")
	}
	directory := filepath.Join(state, "topics")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(normalized))
	lease := filepath.Join(directory, hex.EncodeToString(digest[:])+".json")
	data, err := json.Marshal(activeTopic{Topic: topic, Domain: d.ID, PID: os.Getpid(), StartedAt: time.Now().UTC()})
	if err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(directory, ".topic-*.tmp")
	if err != nil {
		return "", err
	}
	temporaryPath := temporary.Name()
	removeTemporary := func() error {
		if err := os.Remove(temporaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove temporary topic lease %s: %w", temporaryPath, err)
		}
		return nil
	}
	closeAndRemove := func(cause error) error {
		return errors.Join(cause, temporary.Close(), removeTemporary())
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		return "", closeAndRemove(err)
	}
	if err := temporary.Sync(); err != nil {
		return "", closeAndRemove(err)
	}
	if err := temporary.Close(); err != nil {
		return "", errors.Join(err, removeTemporary())
	}
	if err := os.Link(temporaryPath, lease); err != nil {
		if cleanupErr := removeTemporary(); cleanupErr != nil {
			return "", errors.Join(fmt.Errorf("create topic lease: %w", err), cleanupErr)
		}
		if os.IsExist(err) {
			return "", fmt.Errorf("%w: %s", errTopicAlreadyActive, topic)
		}
		return "", fmt.Errorf("create topic lease: %w", err)
	}
	if err := removeTemporary(); err != nil {
		if releaseErr := os.Remove(lease); releaseErr != nil && !errors.Is(releaseErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("remove unreturned topic lease %s: %w", lease, releaseErr))
		}
		return "", err
	}
	return lease, nil
}

// Call while holding topic-selection.lock because this also removes stale leases.
func readActiveTopics(state string) ([]activeTopic, error) {
	directory := filepath.Join(state, "topics")
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	topics := make([]activeTopic, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		topic, found, err := readTopicLease(filepath.Join(directory, entry.Name()), entry.Name())
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		alive, err := topicLeaseOwnerAlive(topic, time.Now())
		if err != nil {
			return nil, err
		}
		if !alive {
			lease := filepath.Join(directory, entry.Name())
			if err := os.Remove(lease); err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("remove stale topic lease %s: %w", lease, err)
			}
			continue
		}
		topics = append(topics, topic)
	}
	slices.SortFunc(topics, func(a, b activeTopic) int {
		if order := strings.Compare(a.Domain, b.Domain); order != 0 {
			return order
		}
		return strings.Compare(a.Topic, b.Topic)
	})
	return topics, nil
}

func readTopicLease(path, name string) (activeTopic, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return activeTopic{}, false, nil
	}
	if err != nil {
		return activeTopic{}, false, err
	}
	var topic activeTopic
	if err := json.Unmarshal(data, &topic); err != nil {
		return activeTopic{}, false, fmt.Errorf("read active topic %s: %w", name, err)
	}
	if topic.Topic == "" || topic.Domain == "" {
		return activeTopic{}, false, fmt.Errorf("active topic %s has incomplete metadata", name)
	}
	return topic, true, nil
}

func topicLeaseOwnerAlive(topic activeTopic, now time.Time) (bool, error) {
	if topic.PID <= 0 {
		return false, nil
	}
	err := syscall.Kill(topic.PID, 0)
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	if err != nil && !errors.Is(err, syscall.EPERM) {
		return false, fmt.Errorf("check topic lease owner %d: %w", topic.PID, err)
	}
	if !topic.StartedAt.IsZero() && now.Sub(topic.StartedAt) > maxTopicLeaseAge {
		return false, nil
	}
	return true, nil
}

func releaseTopic(lease string) error {
	if lease == "" {
		return nil
	}
	if err := os.Remove(lease); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("release topic lease %s: %w", lease, err)
	}
	return nil
}

func acquireTopicSelection(ctx context.Context, state string) (func() error, error) {
	path := filepath.Join(state, "topic-selection.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open topic-selection lock: %w", err)
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			cause := context.Cause(ctx)
			if cause == nil {
				cause = ctx.Err()
			}
			return nil, errors.Join(cause, file.Close())
		}
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			if ctx.Err() != nil {
				cause := context.Cause(ctx)
				if cause == nil {
					cause = ctx.Err()
				}
				unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
				return nil, errors.Join(cause, unlockErr, file.Close())
			}
			return func() error {
				unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
				closeErr := file.Close()
				if unlockErr != nil {
					unlockErr = fmt.Errorf("unlock topic selection: %w", unlockErr)
				}
				if closeErr != nil {
					closeErr = fmt.Errorf("close topic-selection lock: %w", closeErr)
				}
				return errors.Join(unlockErr, closeErr)
			}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return nil, errors.Join(fmt.Errorf("lock topic selection: %w", err), file.Close())
		}
		select {
		case <-ctx.Done():
			cause := context.Cause(ctx)
			if cause == nil {
				cause = ctx.Err()
			}
			return nil, errors.Join(cause, file.Close())
		case <-ticker.C:
		}
	}
}
