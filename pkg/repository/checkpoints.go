package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raphael-foliveira/tca/pkg/models"
)

type CheckpointsRepository interface {
	Create(ctx context.Context, checkpoint models.Checkpoint) (models.Checkpoint, error)
	GetByID(ctx context.Context, id string) (models.Checkpoint, error)
	GetLatestBySessionID(ctx context.Context, sessionID string) (models.Checkpoint, error)
}

var _ CheckpointsRepository = (*pgxCheckpointsRepository)(nil)

type pgxCheckpointsRepository struct {
	db *pgxpool.Pool
}

func NewPGXCheckpointsRepository(db *pgxpool.Pool) CheckpointsRepository {
	return &pgxCheckpointsRepository{
		db: db,
	}
}

const createCheckpoint = `
INSERT INTO 
checkpoints (session_id, context, prompt, response, input_tokens, output_tokens, is_summary)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

`

func (p *pgxCheckpointsRepository) Create(ctx context.Context, checkpoint models.Checkpoint) (models.Checkpoint, error) {
	rows, err := p.db.Query(
		ctx,
		createCheckpoint,
		checkpoint.SessionID,
		checkpoint.Context,
		checkpoint.Prompt,
		checkpoint.Response,
		checkpoint.InputTokens,
		checkpoint.OutputTokens,
		checkpoint.IsSummary,
	)
	if err != nil {
		return models.Checkpoint{}, fmt.Errorf("failed to create checkpoint: %w", err)
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[models.Checkpoint])
}

const getCheckpointByID = `
SELECT *
FROM checkpoints 
WHERE id = $1;
`

func (p *pgxCheckpointsRepository) GetByID(ctx context.Context, id string) (models.Checkpoint, error) {
	rows, err := p.db.Query(
		ctx,
		getCheckpointByID,
		id,
	)
	if err != nil {
		return models.Checkpoint{}, fmt.Errorf("failed to get checkpoint by id: %w", err)
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[models.Checkpoint])
}

const getLatestCheckpointBySessionID = `
WITH ensured AS (
	INSERT INTO sessions (id) VALUES ($1)
	ON CONFLICT (id) DO NOTHING
)
SELECT *
FROM checkpoints
WHERE session_id = $1
ORDER BY created_at DESC
LIMIT 1;
`

func (p *pgxCheckpointsRepository) GetLatestBySessionID(ctx context.Context, sessionID string) (models.Checkpoint, error) {
	rows, err := p.db.Query(
		ctx,
		getLatestCheckpointBySessionID,
		sessionID,
	)
	if err != nil {
		return models.Checkpoint{}, fmt.Errorf("failed to get latest checkpoint by session id: %w", err)
	}
	latestCheckpoint, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[models.Checkpoint])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Checkpoint{
				SessionID: sessionID,
				Context:   []models.ContextMsg{},
			}, nil
		}
		return models.Checkpoint{}, fmt.Errorf("failed to collect latest checkpoint row: %w", err)
	}
	return latestCheckpoint, nil
}
