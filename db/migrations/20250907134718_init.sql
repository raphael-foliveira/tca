-- +goose Up
-- +goose StatementBegin
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now ()
);

CREATE TABLE checkpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    session_id UUID NOT NULL REFERENCES sessions (id),
    context JSONB NOT NULL,
    prompt TEXT NOT NULL,
    response TEXT NOT NULL,
    input_tokens BIGINT,
    output_tokens BIGINT,
    is_summary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now ()
);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE checkpoints;

DROP TABLE sessions;

-- +goose StatementEnd