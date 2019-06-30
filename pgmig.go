package pgmig

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strconv"

	"github.com/jackc/pgx"
	"gopkg.in/birkirb/loggers.v1"
)

// Config holds all config vars
type Config struct {
	Dir string `long:"dir" default:"sql" description:"SQL sources directory"`

	DownloadLimit int64  `long:"download_limit" default:"8" description:"External image size limit (Mb)"`
	PreviewDir    string `long:"preview_dir" default:"data/preview" description:"Preview image destination"`
	PreviewWidth  int    `long:"preview_width" default:"100" description:"Preview image width"`
	PreviewHeight int    `long:"preview_heigth" default:"100" description:"Preview image heigth"`
	UseRandomName bool   `long:"random_name" description:"Do not keep uploaded image filename"`
}

const base = "sql/pgmig-testing/"

// Migrator holds service data
type Migrator struct {
	Config *Config
	Log    loggers.Contextual
}

// New creates an Service object
func New(cfg Config, log loggers.Contextual) *Migrator {
	return &Migrator{&cfg, log}
}

// Run does all work
func (mig *Migrator) Run() error {

	config, err := pgx.ParseEnvLibpq()
	// connStr := "postgres://@localhost/?sslmode=disable"
	// configMore, err := pgx.ParseURI(connStr)
	// config = config.Merge(configMore)
	if err != nil {
		log.Fatal(err)
	}
	buf := []pgx.Notice{}
	doCommit := true
	var cnt int
	var cur int

	config.OnNotice = func(c *pgx.Conn, n *pgx.Notice) {
		if n.Code == "01998" {
			cnt, _ = strconv.Atoi(n.Message)
			cur = 1
			buf = []pgx.Notice{}
		} else if n.Code == "01999" {
			fmt.Printf("(%d/%d) %-20s: Ok\n", cur, cnt, n.Message)
			cur++
			buf = []pgx.Notice{}
		} else if n.Code == "02999" {
			fmt.Printf("(%d/%d) %-20s: Not Ok\n%s\n", cur, cnt, n.Message, n.Detail)
			cur++
			if len(buf) > 0 {
				fmt.Println(buf)
			}
			buf = []pgx.Notice{}
			doCommit = false
		} else {
			//	buf = append(buf, *n)
			fmt.Printf("%s: %s\n", n.Severity, n.Message)

		}

	}

	db, err := pgx.Connect(config)
	if err != nil {
		log.Fatal(err)
	}

	//path := "sql/pgmig"                               //filepath.Join("testdata", schema)
	files, _ := filepath.Glob(base + "[1-7]?_*.sql") // load only files with sources
	fmt.Printf("Found %v\n", files)
	tx, _ := db.Begin()

	for _, f := range files {
		//trimsuffix
		/*
			if !fileUsed() {
				continue
			}
		*/
		//	if strings.Contains(f, "/1") && !(strings.Contains(f, "/18") || strings.Contains(f, "/19")) {
		// skip pkg setup files
		//		continue
		//	}
		fmt.Printf("Load %s\n", f)
		s, err := ioutil.ReadFile(f)
		if err != nil {
			//			return err
			log.Fatalf("%+v", err)
		}
		query := string(s)

		_, err = tx.Exec(query)
		if pgErr, ok := err.(pgx.PgError); ok && pgErr.Code == "P0001" {
			log.Printf("== %s", pgErr.Message)
			doCommit = false

		}
		if err != nil {

			if pgErr, ok := err.(pgx.PgError); !ok || pgErr.Code != "P0001" {
				log.Fatalf(`err => %v, want PgError{Code: "42P01"}`, err)
				doCommit = false
				fmt.Println(buf)
			}
		}

		if !doCommit {
			break
		}

	}
	if doCommit {
		fmt.Println("Commit")
		err = tx.Commit()
	} else {
		fmt.Println("Rollback")
		err = tx.Rollback()
	}
	if err != nil {
		log.Fatalf("EC: %+v", err)
	}
	return nil
}
