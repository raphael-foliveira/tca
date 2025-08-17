package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raphael-foliveira/tca/pkg/models"
)

type SessionsRepository interface {
	GetByID(ctx context.Context, id string) (models.Session, error)
}

var _ SessionsRepository = (*pgxSessionsRepository)(nil)

type pgxSessionsRepository struct {
	db *pgxpool.Pool
}

func NewPGXSessionsRepository(db *pgxpool.Pool) SessionsRepository {
	return &pgxSessionsRepository{
		db: db,
	}
}

const getSessionByID = `
SELECT *
FROM sessions
WHERE id = $1;
`

func (p *pgxSessionsRepository) GetByID(ctx context.Context, id string) (models.Session, error) {
	rows, err := p.db.Query(
		ctx,
		getSessionByID,
		id,
	)
	if err != nil {
		return models.Session{}, fmt.Errorf("failed to get session by id: %w", err)
	}

	newSession, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[models.Session])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Session{}, errors.New("session not found")
		}
		return models.Session{}, fmt.Errorf("failed to collect session row: %w", err)
	}
	return newSession, nil
}

const createSession = `
INSERT INTO sessions (id) VALUES ($1)
RETURNING *;
`

func (p pgxSessionsRepository) Create(ctx context.Context, session models.Session) (models.Session, error) {
	rows, err := p.db.Query(
		ctx,
		createSession,
		session.ID,
	)
	if err != nil {
		return models.Session{}, fmt.Errorf("failed to create session: %w", err)
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[models.Session])
}
