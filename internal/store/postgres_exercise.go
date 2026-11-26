package store

import (
	"context"
	"database/sql"
	"fmt"
	"main/internal/models"

	"github.com/lib/pq"
)

type postgresExerciseStore struct {
	db *sql.DB
}

func NewPostgressExerciseStore(db *sql.DB) *postgresExerciseStore {
	return &postgresExerciseStore{
		db: db,
	}
}

// SearchExercises implements [models.ExerciseRepository].
func (p *postgresExerciseStore) SearchExercises(ctx context.Context, searchTerm string, limit int) ([]*models.Exercise, error) {
	query := `SELECT e.id, e.name, e.equipment, e.primary_muscle, e.secondary_muscles, e.steps, e.created_at, e.updated_at
		FROM exercises e LEFT JOIN exercise_aliases a ON e.id = a.exercise_id
		WHERE e.deleted_at IS NULL
		AND (lower(e.name) LIKE '%' || $1 || '%' OR a.alias LIKE '%' || $1 || '%'
		OR $1 <% lower(e.name) OR $1 <% a.alias)
		GROUP BY e.id
		ORDER BY MIN(
		CASE
			WHEN lower(e.name) = $1 OR a.alias = $1
		THEN 0 -- Exact Match on name or alias
			WHEN lower(e.name) LIKE $1 || '%' OR a.alias LIKE $1 || '%' 
		THEN 1
			WHEN $1 <% lower(e.name) or $1 <% a.alias
		THEN 2
		ELSE 3
		END), MAX(GREATEST(word_similarity($1, lower(e.name)), word_similarity($1, coalesce(a.alias, '')))) DESC, e.name
		LIMIT $2
	`
	exercises := make([]*models.Exercise, 0)

	// `<%` matches when word_similarity >= pg_trgm.word_similarity_threshold (default 0.6). At 0.6 short words
	// with one letter missing don't match (word_similarity('dedlift', 'deadlift') = 0.545), so lower it to 0.5.
	// SET LOCAL only lasts until the transaction ends, so the pooled connection goes back unchanged.
	// The transaction is never committed: it only reads, and the deferred Rollback ends it.
	tx, err := p.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("exercise search begin tx failed: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SET LOCAL pg_trgm.word_similarity_threshold = 0.5`); err != nil {
		return nil, fmt.Errorf("exercise search set similarity threshold failed: %w", err)
	}

	rows, err := tx.QueryContext(ctx, query, searchTerm, limit)
	if err != nil {
		return nil, fmt.Errorf("exercise search query failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		e := &models.Exercise{}
		err := rows.Scan(
			&e.ID,
			&e.Name,
			&e.Equipment,
			&e.PrimaryMuscle,
			pq.Array(&e.SecondaryMuscles),
			pq.Array(&e.Steps),
			&e.CreatedAt,
			&e.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("exercise search scan failed: %w", err)
		}
		exercises = append(exercises, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("exercise search error: %w", err)
	}

	return exercises, nil

}
