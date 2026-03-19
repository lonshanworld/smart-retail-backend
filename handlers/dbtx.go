package handlers

import (
	"context"

	"github.com/jackc/pgx/v4"
)

// DBRow is a minimal wrapper to support Scan in tests and real pgx rows.
type DBRow interface {
	Scan(dest ...interface{}) error
}

// DBTx defines the minimal methods used by the offline sync logic.
type DBTx interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) DBRow
	Exec(ctx context.Context, sql string, args ...interface{}) (int64, error)
}

// pgxRowAdapter adapts pgx.Row to DBRow.
type pgxRowAdapter struct {
	row pgx.Row
}

func (r pgxRowAdapter) Scan(dest ...interface{}) error { return r.row.Scan(dest...) }

// pgxTxAdapter adapts pgx.Tx to DBTx.
type pgxTxAdapter struct {
	tx pgx.Tx
}

func (p pgxTxAdapter) QueryRow(ctx context.Context, sql string, args ...interface{}) DBRow {
	return pgxRowAdapter{row: p.tx.QueryRow(ctx, sql, args...)}
}

func (p pgxTxAdapter) Exec(ctx context.Context, sql string, args ...interface{}) (int64, error) {
	ct, err := p.tx.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}
