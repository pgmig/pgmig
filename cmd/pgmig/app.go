package main

//go:generate gitinfo ../../sql/pgmig
//go:generate go-imbed -union-fs ../../sql internal/sql

import (
	"context"
	"log"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	// TODO	"github.com/jackc/pgx/v4/log/logrusadapter"

	"github.com/pgmig/pgmig"
	"github.com/pgmig/pgmig/cmd/pgmig/internal/sql"
)

// SQLRoot hardcoded in go:generate
const SQLRoot = "sql"

// pgmigFileSystem used for conversion from sql.FileSystem to pgmig.FileSystem
type pgmigFileSystem struct {
	sql.FileSystem
}

func (fs pgmigFileSystem) Open(name string) (pgmig.File, error) { return fs.FileSystem.Open(name) }

func run(exitFunc func(code int)) {
	var err error
	var cfg *Config
	defer func() { shutdown(exitFunc, err) }()
	cfg, err = setupConfig()
	if err != nil {
		return
	}
	l := setupLog(cfg)

	fs, e := sql.NewUnionFS(SQLRoot)
	if e != nil {
		err = e
		return
	}

	cfg.Mig.GitInfo.Root = SQLRoot
	mig := pgmig.New(cfg.Mig, l, pgmigFileSystem{fs}, "")

	config, err := pgx.ParseConfig(cfg.DSN)
	if err != nil {
		return
	}
	//	config.Logger = logrusadapter.NewLogger(l)
	config.OnNotice = func(c *pgconn.PgConn, n *pgconn.Notice) {
		mig.ProcessNotice(n.Code, n.Message, n.Detail)
	}

	// TODO: statement_cache_mode = "describe"
	config.BuildStatementCache = nil // disable stmt cache
	ctx := context.Background()
	dbh, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return
	}

	tx, err := dbh.Begin(ctx)
	if err != nil {
		return
	}
	defer func() {
		er := tx.Rollback(ctx)
		if er != nil && er != pgx.ErrTxClosed {
			mig.Log.Error(er)
		}
	}()

	commit, err := mig.Run(tx, cfg.Args.Command, cfg.Args.Packages)
	if err == nil && *commit {
		log.Println("Commit")
		err = tx.Commit(ctx)
	}
	if err == nil || err != pgx.ErrTxClosed { // shutdown shows error otherwise
		log.Printf("Saved: %v", *commit)
	}
}

// exit after deferred cleanups have run
func shutdown(exitFunc func(code int), e error) {
	if e != nil {
		var code int
		switch e {
		case ErrGotHelp:
			code = 3
		case ErrBadArgs:
			code = 2
		default:
			if e != pgx.ErrTxClosed {
				code = 1
				log.Printf("Run error: %+v", e)
			}
		}
		exitFunc(code)
	}
}
