package pgmig

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx"
	"github.com/pkg/errors"
	"gopkg.in/birkirb/loggers.v1"
)

// Config holds all config vars
type Config struct {
	Dir      string `long:"dir" default:"sql" description:"SQL sources directory"`
	NoCommit bool   `long:"nocommit" description:"Do not commit work"`
}

const (
	pgStatusTestCount = "01998"
	pgStatusTestOk    = "01999"
	pgStatusTestFail  = "02999"
)

// FileSystem holds all of used filesystem access methods
type FileSystem interface {
	Walk(root string, walkFn filepath.WalkFunc) error
	Open(name string) (http.File, error)
}

// Migrator holds service data
type Migrator struct {
	Config *Config
	Log    loggers.Contextual
	FS     FileSystem
}

// New creates an Service object
func New(cfg Config, log loggers.Contextual, fs *FileSystem) *Migrator {
	mig := Migrator{Config: &cfg, Log: log}
	if fs == nil {
		mig.FS = defaultFS{}
	} else {
		mig.FS = *fs
	}
	return &mig
}

type Command struct {
	Mask           []string
	ReverseCommand string
}

var commandDef = map[string]Command{
	"create":    Command{Mask: []string{"[1-7]?_*.sql"}},
	"create_ok": Command{Mask: []string{"[1-6]?_*.sql", "7[0-8]_*.sql"}},
	"test":      Command{Mask: []string{"7?_*.sql"}},
	"drop":      Command{Mask: []string{"8?_*.sql"}},
}

var errCancel = errors.New("Rollback")

/*
func (mig Migrator) NoticeFunc(notices *[]pgx.Notice) func(c *pgx.Conn, n *pgx.Notice) {
	return func(c *pgx.Conn, n *pgx.Notice) {
		if n.Code == pgStatusTestCount {
			cnt, _ = strconv.Atoi(n.Message)
			cur = 1
			notices = []pgx.Notice{}
		} else if n.Code == pgStatusTestOk {
			fmt.Printf("(%d/%d) %-20s: Ok\n", cur, cnt, n.Message)
			cur++
			notices = []pgx.Notice{}
		} else if n.Code == pgStatusTestFail {
			fmt.Printf("(%d/%d) %-20s: Not Ok\n%s\n", cur, cnt, n.Message, n.Detail)
			cur++
			if len(notices) > 0 {
				fmt.Println(notices)
			}
			notices = []pgx.Notice{}
			doCommit = false
		} else {
			//	notices = append(notices, *n)
			fmt.Printf("%s: %s\n", n.Severity, n.Message)
		}
	}
}
*/

// Run does all work
func (mig *Migrator) Run(command string, packages []string) (*bool, error) {

	config, err := pgx.ParseEnvLibpq()
	// connStr := "postgres://@localhost/?sslmode=disable"
	// configMore, err := pgx.ParseURI(connStr)
	// config = config.Merge(configMore)
	if err != nil {
		return nil, errors.Wrap(err, "DB parse ENV")
	}
	buf := []pgx.Notice{}
	noCommit := false
	var cnt int
	var cur int

	config.OnNotice = func(c *pgx.Conn, n *pgx.Notice) {
		if n.Code == pgStatusTestCount {
			cnt, _ = strconv.Atoi(n.Message)
			cur = 1
			buf = []pgx.Notice{}
		} else if n.Code == pgStatusTestOk {
			fmt.Printf("(%d/%d) %-20s: Ok\n", cur, cnt, n.Message)
			cur++
			buf = []pgx.Notice{}
		} else if n.Code == pgStatusTestFail {
			fmt.Printf("(%d/%d) %-20s: Not Ok\n%s\n", cur, cnt, n.Message, n.Detail)
			cur++
			if len(buf) > 0 {
				fmt.Println(buf)
			}
			buf = []pgx.Notice{}
			noCommit = true
		} else {
			//	buf = append(buf, *n)
			fmt.Printf(">>> %s: %s\n", n.Severity, n.Message)
		}
	}

	def, ok := commandDef[command]
	if !ok {
		return nil, errors.New("Unknown command " + command)
	}
	var cmdBefore *Command
	if def.ReverseCommand != "" {
		cmd, ok := commandDef[def.ReverseCommand]
		if !ok {
			return nil, errors.New("Unknown reverse command " + command)
		}
		*cmdBefore = cmd
	}

	db, err := pgx.Connect(config)
	if err != nil {
		return nil, errors.Wrap(err, "DB connect")
	}

	tx, _ := db.Begin()
	defer tx.Rollback()

	if cmdBefore != nil {
		pkg := packages
		sort.Sort(sort.Reverse(sort.StringSlice(pkg)))
		err = mig.execFiles(tx, cmdBefore.Mask, pkg)
		if err != nil {
			return nil, errors.Wrap(err, "Reverse command error")
		}
	}
	err = mig.execFiles(tx, def.Mask, packages)
	if err != nil && err != errCancel {
		return nil, err
	}
	var rv bool
	if err != nil || noCommit || mig.Config.NoCommit {
		fmt.Println("Rollback")
		err = tx.Rollback()
	} else {
		fmt.Println("Commit")
		err = tx.Commit()
		rv = true
	}
	if err != nil {
		return nil, errors.Wrap(err, "End work error")
	}
	return &rv, nil
}

func (mig Migrator) execFiles(tx *pgx.Tx, mask []string, packages []string) error {

	for _, pkg := range packages {
		fmt.Printf("** %s\n", pkg)

		root := filepath.Join(mig.Config.Dir, pkg)
		var files []string
		err := mig.FS.Walk(root, mig.WalkerFunc(mask, &files))
		if err != nil {
			return errors.Wrap(err, "Walk error")
		}

		// fmt.Printf("Found %v\n", files)
		sort.StringSlice(files).Sort()
		for _, file := range files {
			f := filepath.Join(root, file)
			fmt.Printf("Load %s\n", file)
			s, err := ioutil.ReadFile(f)
			if err != nil {
				return errors.Wrap(err, "Reading "+file)
			}
			query := string(s)
			//fmt.Print(query)
			_, err = tx.Exec(query)
			if err == nil {
				continue
			}
			pgErr, ok := err.(pgx.PgError)
			if !ok {
				return errors.Wrap(err, "System error")
			}
			if pgErr.Code == "P0001" {
				log.Printf("App error: %s", pgErr.Message)
			} else {
				lineNo := strings.Count(string([]rune(query)[:pgErr.Position]), "\n") + 1
				fmt.Printf("%s:%d [%s %s] %s\n", file, lineNo, pgErr.Severity, pgErr.Code, pgErr.Message)
				if pgErr.Detail != "" {
					fmt.Printf("DETAIL: %s\n", pgErr.Detail)
				}
				if pgErr.Hint != "" {
					fmt.Printf("HINT: %s\n", pgErr.Hint)
				}
			}
			return errCancel
		}
	}
	return nil
}

func (mig Migrator) WalkerFunc(mask []string, files *[]string) func(path string, f os.FileInfo, err error) error {
	return func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if f.IsDir() {
			return nil
		}
		for _, m := range mask {
			matched, err := filepath.Match(m, f.Name())
			if err != nil {
				return err
			}
			if matched {
				*files = append(*files, f.Name())
				break
			}

		}
		return nil
	}
}
