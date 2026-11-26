package service

import (
	"context"
	"errors"
	"main/internal/apperrors"
	"main/internal/models"
	"strings"
	"testing"
)

// MockExerciseRepo fakes the exercise store and records what the service passed to it.
type MockExerciseRepo struct {
	results []*models.Exercise
	err     error

	called   bool
	gotTerm  string
	gotLimit int
}

// SearchExercises implements [models.ExerciseRepository].
func (m *MockExerciseRepo) SearchExercises(ctx context.Context, searchTerm string, limit int) ([]*models.Exercise, error) {
	m.called = true
	m.gotTerm = searchTerm
	m.gotLimit = limit
	return m.results, m.err
}

func TestSearchExercisesPreparesTerm(t *testing.T) {
	tests := []struct {
		name     string
		term     string
		wantTerm string
	}{
		{"lowercases", "Bench Press", "bench press"},
		{"trims", "  rdl \t", "rdl"},
		{"escapes percent", "50%", `50\%`},
		{"escapes underscore", "a_b", `a\_b`},
		{"escapes backslash first", `a\b`, `a\\b`},
		{"two letter alias", "DL", "dl"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &MockExerciseRepo{results: []*models.Exercise{}}
			svc := NewExerciseService(repo)

			if _, err := svc.SearchExercises(context.Background(), tc.term, 20); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.gotTerm != tc.wantTerm {
				t.Errorf("store got term %q, want %q", repo.gotTerm, tc.wantTerm)
			}
		})
	}
}

func TestSearchExercisesLimit(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		wantLimit int
	}{
		{"passes through", 20, 20},
		{"one", 1, 1},
		{"at cap", 50, 50},
		{"capped", 500, 50},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &MockExerciseRepo{results: []*models.Exercise{}}
			svc := NewExerciseService(repo)

			if _, err := svc.SearchExercises(context.Background(), "bench", tc.limit); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.gotLimit != tc.wantLimit {
				t.Errorf("store got limit %d, want %d", repo.gotLimit, tc.wantLimit)
			}
		})
	}
}

func TestSearchExercisesBadRequest(t *testing.T) {
	tests := []struct {
		name      string
		term      string
		limit     int
		wantField string
	}{
		{"empty query", "", 20, "q"},
		{"one character", "a", 20, "q"},
		{"only whitespace", "     ", 20, "q"},
		{"one character after trimming", "  a  ", 20, "q"},
		{"too long", strings.Repeat("a", 51), 20, "q"},
		{"too long in characters, not bytes", strings.Repeat("é", 51), 20, "q"},
		{"negative limit", "bench", -1, "limit"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &MockExerciseRepo{}
			svc := NewExerciseService(repo)

			results, err := svc.SearchExercises(context.Background(), tc.term, tc.limit)
			if !apperrors.Is(err, apperrors.BadRequest) {
				t.Fatalf("expected BadRequest, got %v", err)
			}
			var kerr *apperrors.KError
			errors.As(err, &kerr)
			if len(kerr.Fields) != 1 || kerr.Fields[0].Field != tc.wantField {
				t.Errorf("expected one field error on %q, got %+v", tc.wantField, kerr.Fields)
			}
			if results != nil {
				t.Errorf("expected nil results, got %v", results)
			}
			if repo.called {
				t.Error("store should not be called for an invalid request")
			}
		})
	}
}

func TestSearchExercisesMultibyteWithinLimits(t *testing.T) {
	// 50 two-byte characters is 100 bytes but still within the 50 character limit.
	repo := &MockExerciseRepo{results: []*models.Exercise{}}
	svc := NewExerciseService(repo)

	if _, err := svc.SearchExercises(context.Background(), strings.Repeat("é", 50), 20); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.SearchExercises(context.Background(), "éé", 20); err != nil {
		t.Fatalf("two multibyte characters should be long enough, got: %v", err)
	}
}

func TestSearchExercisesReturnsResults(t *testing.T) {
	want := []*models.Exercise{{ID: "romanian-deadlift"}, {ID: "dumbbell-romanian-deadlift"}}
	repo := &MockExerciseRepo{results: want}
	svc := NewExerciseService(repo)

	got, err := svc.SearchExercises(context.Background(), "rdl", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("expected the store's results unchanged, got %v", got)
	}
}

func TestSearchExercisesStoreError(t *testing.T) {
	storeErr := errors.New("connection refused")
	repo := &MockExerciseRepo{err: storeErr}
	svc := NewExerciseService(repo)

	results, err := svc.SearchExercises(context.Background(), "bench", 20)
	if !errors.Is(err, storeErr) {
		t.Fatalf("expected the store error to be passed up, got %v", err)
	}
	var kerr *apperrors.KError
	if errors.As(err, &kerr) {
		t.Errorf("store errors should not get a kind (Handle turns them into Internal), got %v", kerr.Kind.Code)
	}
	if results != nil {
		t.Errorf("expected nil results, got %v", results)
	}
}
