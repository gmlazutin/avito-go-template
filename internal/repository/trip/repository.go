package trip

import (
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

var sqlBuilder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

type Repository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *Repository {
	return &Repository{pool: pool, queryTimeout: queryTimeout}
}
