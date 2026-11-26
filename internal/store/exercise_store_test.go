package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests run against the seeded catalog (migrations/..._seed_exercises.sql).
// clearTables only truncates users, so the seed survives between tests.
// Search terms are passed the way the service hands them over: trimmed, lowercased and LIKE-escaped.

func searchIDs(t *testing.T, term string, limit int) []string {
	t.Helper()
	exerciseStore := NewPostgressExerciseStore(testDB)
	results, err := exerciseStore.SearchExercises(context.Background(), term, limit)
	require.NoError(t, err)
	ids := make([]string, len(results))
	for i, e := range results {
		ids[i] = e.ID
	}
	return ids
}

func TestExerciseSeedLoaded(t *testing.T) {
	var exercises, aliases int
	require.NoError(t, testDB.QueryRow(`SELECT count(*) FROM exercises`).Scan(&exercises))
	require.NoError(t, testDB.QueryRow(`SELECT count(*) FROM exercise_aliases`).Scan(&aliases))
	assert.Equal(t, 198, exercises)
	assert.Equal(t, 480, aliases)
}

func TestSearchExercisesRanking(t *testing.T) {
	tests := []struct {
		name    string
		term    string
		firstID string
	}{
		{"exact name", "barbell bench press", "barbell-bench-press"},
		{"exact alias", "rdl", "romanian-deadlift"},
		{"exact alias beats other names containing it", "bench press", "barbell-bench-press"},
		{"exact alias shorter than three letters", "dl", "barbell-deadlift"},
		{"exact alias beats prefix matches", "squat", "barbell-back-squat"},
		{"alias prefix", "deadl", "barbell-deadlift"},
		{"alias without space", "skullcrusher", "skull-crusher"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ids := searchIDs(t, tc.term, 20)
			require.NotEmpty(t, ids)
			assert.Equal(t, tc.firstID, ids[0], "results: %v", ids)
		})
	}
}

// Typos are matched by `<%`, which the store runs with pg_trgm.word_similarity_threshold = 0.5.
func TestSearchExercisesTypo(t *testing.T) {
	tests := []struct {
		name   string
		term   string
		wantID string
	}{
		{"missing letter in a short word", "dedlift", "barbell-deadlift"},
		{"missing space", "legpress", "leg-press"},
		{"missing letter", "skul crusher", "skull-crusher"},
		{"swapped letters", "romanain deadlift", "romanian-deadlift"},
		{"extra letter", "squaat", "barbell-back-squat"},
		{"truncated", "bench pres", "barbell-bench-press"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ids := searchIDs(t, tc.term, 20)
			assert.Contains(t, ids, tc.wantID)
		})
	}
}

func TestSearchExercisesLooseMatchesRankLast(t *testing.T) {
	// The 0.5 threshold lets weaker matches in (e.g. "lateral raise" for "calf raise"); they must sort after the real ones.
	ids := searchIDs(t, "calf raise", 20)
	require.GreaterOrEqual(t, len(ids), 4)
	assert.ElementsMatch(t,
		[]string{"standing-calf-raise", "leg-press-calf-raise", "seated-calf-raise", "smith-machine-calf-raise"},
		ids[:4], "results: %v", ids)
}

func TestSearchExercisesThresholdDoesNotLeak(t *testing.T) {
	// SET LOCAL must end with the search's transaction, not stay on the pooled connection.
	exerciseStore := NewPostgressExerciseStore(testDB)
	testDB.SetMaxOpenConns(1)
	t.Cleanup(func() { testDB.SetMaxOpenConns(0) })

	_, err := exerciseStore.SearchExercises(context.Background(), "dedlift", 20)
	require.NoError(t, err)

	var threshold string
	require.NoError(t, testDB.QueryRow(`SHOW pg_trgm.word_similarity_threshold`).Scan(&threshold))
	assert.Equal(t, "0.6", threshold)
}

func TestSearchExercisesGroupsAliasMatches(t *testing.T) {
	// "rdl" matches several aliases of the same exercises; each exercise must appear once.
	ids := searchIDs(t, "rdl", 50)
	seen := map[string]bool{}
	for _, id := range ids {
		assert.False(t, seen[id], "duplicate exercise %s in %v", id, ids)
		seen[id] = true
	}
	assert.Contains(t, ids, "dumbbell-romanian-deadlift")
	assert.Contains(t, ids, "single-leg-romanian-deadlift")
}

func TestSearchExercisesNoResults(t *testing.T) {
	exerciseStore := NewPostgressExerciseStore(testDB)
	results, err := exerciseStore.SearchExercises(context.Background(), "xyzq", 20)
	require.NoError(t, err)
	assert.NotNil(t, results, "no results must be an empty slice so it encodes as []")
	assert.Empty(t, results)
}

func TestSearchExercisesEscapedWildcards(t *testing.T) {
	// The service escapes % and _ so they match literally; no exercise name contains them.
	assert.Empty(t, searchIDs(t, `\%`, 20))
	assert.Empty(t, searchIDs(t, `\_\_`, 20))
}

func TestSearchExercisesLimit(t *testing.T) {
	ids := searchIDs(t, "squat", 3)
	assert.Len(t, ids, 3)
	assert.Equal(t, "barbell-back-squat", ids[0])
}

func TestSearchExercisesFewerMatchesThanLimit(t *testing.T) {
	ids := searchIDs(t, "zercher", 50)
	assert.Equal(t, []string{"zercher-squat"}, ids)
}

func TestSearchExercisesNameAndAliasMatchSameExercise(t *testing.T) {
	// "skull crusher" is the name of one exercise and an alias of the same one; it must appear once, first.
	ids := searchIDs(t, "skull crusher", 50)
	require.NotEmpty(t, ids)
	assert.Equal(t, "skull-crusher", ids[0])
	count := 0
	for _, id := range ids {
		if id == "skull-crusher" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestSearchExercisesCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results, err := NewPostgressExerciseStore(testDB).SearchExercises(ctx, "bench", 20)
	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestSearchExercisesSkipsDeleted(t *testing.T) {
	_, err := testDB.Exec(`UPDATE exercises SET deleted_at = now() WHERE id = 'romanian-deadlift'`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := testDB.Exec(`UPDATE exercises SET deleted_at = NULL WHERE id = 'romanian-deadlift'`)
		require.NoError(t, err)
	})

	ids := searchIDs(t, "rdl", 50)
	assert.NotContains(t, ids, "romanian-deadlift")
}

func TestSearchExercisesScansAllFields(t *testing.T) {
	exerciseStore := NewPostgressExerciseStore(testDB)
	results, err := exerciseStore.SearchExercises(context.Background(), "barbell bench press", 1)
	require.NoError(t, err)
	require.Len(t, results, 1)

	e := results[0]
	assert.Equal(t, "barbell-bench-press", e.ID)
	assert.Equal(t, "Barbell Bench Press", e.Name)
	assert.Equal(t, "barbell", e.Equipment)
	assert.Equal(t, "chest", e.PrimaryMuscle)
	assert.NotEmpty(t, e.SecondaryMuscles)
	assert.NotEmpty(t, e.Steps)
	assert.False(t, e.CreatedAt.IsZero())
	assert.False(t, e.UpdatedAt.IsZero())
}
