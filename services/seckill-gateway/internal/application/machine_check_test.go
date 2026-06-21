package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"seckill-gateway-service/internal/config"
)

func TestJavaMachineCheckerChallengeAndCheckOnce(t *testing.T) {
	store := newFakeMachineStore()
	checker := NewJavaMachineChecker(config.MachineCheckConfig{
		RandomLength: 4,
		TTL:          2 * time.Second,
	}, store)
	checker.now = func() time.Time { return time.UnixMilli(1001) }
	checker.random = func(length int) (string, error) {
		if length != 4 {
			t.Fatalf("random length = %d, want 4", length)
		}
		return "ABCD", nil
	}

	challenge, err := checker.Challenge(context.Background(), 7)
	if err != nil {
		t.Fatalf("Challenge returned error: %v", err)
	}
	if challenge.Result != "ABCD" || challenge.Key != 1001 {
		t.Fatalf("challenge = %+v, want ABCD/1001", challenge)
	}
	key := "seckill:machine:check:7"
	if got := store.values[key]; got != "AzCD" {
		t.Fatalf("stored token = %q, want AzCD", got)
	}
	if got := store.ttls[key]; got != 2*time.Second {
		t.Fatalf("ttl = %s, want 2s", got)
	}
	if checker.Check(context.Background(), 7, "wrong") {
		t.Fatal("Check returned true for wrong token")
	}
	if _, ok := store.values[key]; !ok {
		t.Fatal("wrong token deleted stored challenge")
	}
	if !checker.Check(context.Background(), 7, "AzCD") {
		t.Fatal("Check returned false for matching token")
	}
	if _, ok := store.values[key]; ok {
		t.Fatal("matching token was not deleted")
	}
	if checker.Check(context.Background(), 7, "AzCD") {
		t.Fatal("Check returned true for reused token")
	}
}

func TestJavaMachineCheckerChallengePropagatesStoreError(t *testing.T) {
	store := newFakeMachineStore()
	store.setErr = errors.New("down")
	checker := NewJavaMachineChecker(config.MachineCheckConfig{}, store)
	checker.random = func(int) (string, error) { return "ABCDEFGHIJKLMNOP", nil }

	_, err := checker.Challenge(context.Background(), 7)
	if err == nil {
		t.Fatal("Challenge returned nil error, want store error")
	}
}

type fakeMachineStore struct {
	values map[string]string
	ttls   map[string]time.Duration
	setErr error
	getErr error
	delErr error
}

func newFakeMachineStore() *fakeMachineStore {
	return &fakeMachineStore{
		values: map[string]string{},
		ttls:   map[string]time.Duration{},
	}
}

func (s *fakeMachineStore) Set(_ context.Context, key string, value string, ttl time.Duration) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.values[key] = value
	s.ttls[key] = ttl
	return nil
}

func (s *fakeMachineStore) Get(_ context.Context, key string) (string, error) {
	if s.getErr != nil {
		return "", s.getErr
	}
	return s.values[key], nil
}

func (s *fakeMachineStore) Delete(_ context.Context, key string) error {
	if s.delErr != nil {
		return s.delErr
	}
	delete(s.values, key)
	delete(s.ttls, key)
	return nil
}
