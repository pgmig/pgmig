package main

//go:generate gitinfo ../../sql/pgmig

// embedFS by https://github.com/growler/go-imbed
//go:generate go-imbed -union-fs -no-http-handler ../../sql internal/sql

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v4"
	// TODO	"github.com/jackc/pgx/v4/log/logrusadapter"

	"github.com/pgmig/pgmig"
	"github.com/pgmig/pgmig/cmd/pgmig/internal/sql"
)

// Config holds all config vars
type Config struct {
	Flags
	DSN  string `long:"dsn" default:"" description:"Database URL"`
	Args struct {
		//nolint:staticcheck // Multiple struct tag "choice" is allowed
		Command  string   `choice:"init" choice:"test" choice:"drop" choice:"erase" choice:"reinit" description:"init|test|drop|erase|reinit"`
		Packages []string `description:"dirnames under SQL sources directory in create order"`
	} `positional-args:"yes" required:"yes"`
	Mig pgmig.Config `group:"Migrator Options" namespace:"mig"`

	// SQL packages root. Not used with embedded fs
	// SQLRoot        string            `long:"sql" default:"sql" description:"SQL sources directory"` // TODO: pkg/*/sql

}

// Actual version value will be set at build time
var version = "0.0-dev"

// SQLRoot hardcoded in go:generate
const SQLRoot = "sql"

// pgmigFileSystem used for conversion from sql.FileSystem to pgmig.FileSystem
type pgmigFileSystem struct {
	sql.FileSystem
}

func (fs pgmigFileSystem) Open(name string) (pgmig.File, error) { return fs.FileSystem.Open(name) }

// Run app and exit via given exitFunc
func Run(exitFunc func(code int)) {
	cfg, err := SetupConfig()
	log := SetupLog(err != nil || cfg.Debug)
	defer func() { Shutdown(exitFunc, err, log) }()
	log.Info("pgmig. Postgresql drop/create migrations.", "v", version)
	if err != nil || cfg.Version {
		return
	}

	fs, e := sql.NewUnionFS(SQLRoot)
	if e != nil {
		err = e
		return
	}

	cfg.Mig.GitInfo.Root = SQLRoot
	mig := pgmig.New(log, cfg.Mig, pgmigFileSystem{fs}, "")

	ctx := context.Background()
	dbh, e := mig.Connect(ctx, cfg.DSN)
	if e != nil {
		err = e
		return
	}
	err = dbh.Ping(ctx)
	if err != nil {
		log.Error(err, ">>>>>>>>>")
		return
	}
	tx, e := dbh.Begin(ctx)
	if e != nil {
		err = e
		return
	}
	defer func() {
		er := tx.Rollback(ctx)
		if er != nil && er != pgx.ErrTxClosed {
			mig.Log.Error(er, "Run error")
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go mig.PrintMessages(&wg)
	commit, err := mig.Run(tx, cfg.Args.Command, cfg.Args.Packages)
	if err == nil && *commit {
		err = tx.Commit(ctx)
	}
	close(mig.MessageChan)
	wg.Wait()
	if err == nil || err != pgx.ErrTxClosed { // shutdown shows error otherwise
		log.Info("Saved", "commit", *commit)
	}
}
