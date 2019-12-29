package main

//go:generate gitinfo ../../sql/pgmig

// embedFS by https://github.com/growler/go-imbed
//go:generate go-imbed -union-fs -no-http-handler ../../sql internal/sql

import (
	"context"
	"log"
	"sync"

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

	ctx := context.Background()
	dbh, err := mig.Connect(cfg.DSN)
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

	var wg sync.WaitGroup
	wg.Add(1)
	go mig.PrintMessages(&wg)
	commit, err := mig.Run(tx, cfg.Args.Command, cfg.Args.Packages)
	if err == nil && *commit {
		log.Println("Commit")
		err = tx.Commit(ctx)
	}
	close(mig.MessageChan)
	wg.Wait()
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
