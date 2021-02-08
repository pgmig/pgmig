package pgmig

import (
	"context"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
)

// Connect makes connect to DB and returns ins handle
func (mig *Migrator) Connect(ctx context.Context, dsn string) (*pgx.Conn, error) {
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	//	config.Logger = logrusadapter.NewLogger(l)
	config.OnNotice = func(c *pgconn.PgConn, n *pgconn.Notice) {
		mig.ProcessNotice(n.Code, n.Message, n.Detail)
	}

	// TODO: statement_cache_mode = "describe"
	config.BuildStatementCache = nil // disable stmt cache for `reinit pgmig`
	return pgx.ConnectConfig(ctx, config)
}
