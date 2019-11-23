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
	"sync"

	"github.com/jackc/pgx"
	"github.com/pkg/errors"
	"gopkg.in/birkirb/loggers.v1"
)

// Config holds all config vars
type Config struct {
	Dir      string `long:"dir" default:"sql" description:"SQL sources directory"` // TODO: pkg/*/sql
	NoCommit bool   `long:"nocommit" description:"Do not commit work"`
	ListOnly bool   `long:"listonly" description:"Show file list and exit"`
	// TODO: SearchPath

	NoHooks    bool   `long:"nohooks" description:"Do not call before/after hooks"`
	HookBefore string `long:"hook_before" default:"op_before" description:"Func called before command for every pkg"`
	HookAfter  string `long:"hook_after" default:"op_after" description:"Func called after command for every pkg"`

	InitIncludes  []string `long:"init" default:"*.sql" default:"!*.drop.sql" default:"!*.erase.sql" description:"File masks for init command"`
	TestIncludes  []string `long:"test" default:"*.test.sql" description:"File masks for test command"`
	NewIncludes   []string `long:"new" default:"*.new.sql" description:"File masks loaded on init if package is new"`
	DropIncludes  []string `long:"drop" default:"*.drop.sql" description:"File masks for drop command"`
	EraseIncludes []string `long:"erase" default:"*.erase.sql" default:"*.drop.sql" description:"File masks for drop command"`
	OnceIncludes  []string `long:"once" default:"*.once.sql" description:"File masks loaded once on init"`
}

// FileSystem holds all of used filesystem access methods
type FileSystem interface {
	Walk(root string, walkFn filepath.WalkFunc) error
	Open(name string) (http.File, error)
}

// Migrator holds service data
type Migrator struct {
	Config    *Config
	Log       loggers.Contextual
	FS        FileSystem
	noCommit  bool
	isNew     bool
	isNewLock sync.RWMutex
}

var errCancel = errors.New("Rollback")

const (
	pgStatusTestCount       = "01998"
	pgStatusTestOk          = "01999"
	pgStatusTestFail        = "02999"
	pgStatusDuplicateSchema = "42P06"
)

// New creates an Service object
func New(cfg Config, log loggers.Contextual, fs *FileSystem) *Migrator {
	mig := Migrator{Config: &cfg, Log: log}
	if fs == nil {
		mig.FS = defaultFS{}
	} else {
		mig.FS = *fs
	}
	mig.Log.Debugf("CFG: %#v\n", cfg)
	return &mig
}

func (mig *Migrator) SetNoCommit(commit bool) {
	// TODO: locks
	mig.noCommit = commit
}

func (mig *Migrator) NoCommit() bool {
	// TODO: locks
	return mig.noCommit
}

func (mig *Migrator) NoticeFunc() func(c *pgx.Conn, n *pgx.Notice) {
	var cnt int
	var cur int
	return func(c *pgx.Conn, n *pgx.Notice) {
		if n.Code == pgStatusTestCount {
			cnt, _ = strconv.Atoi(n.Message)
			cur = 1
			//			notices = []pgx.Notice{}
		} else if n.Code == pgStatusTestOk {
			fmt.Printf("(%d/%d) %-20s: Ok\n", cur, cnt, n.Message)
			cur++
			//			notices = []pgx.Notice{}
		} else if n.Code == pgStatusTestFail {
			fmt.Printf("(%d/%d) %-20s: Not Ok\n%s\n", cur, cnt, n.Message, n.Detail)
			cur++
			//			if len(notices) > 0 {
			//				fmt.Println(notices)
			//			}
			//			notices = []pgx.Notice{}
			mig.SetNoCommit(true)
		} else if n.Code == pgStatusDuplicateSchema {
			fmt.Printf("Schema exists already\n")
			mig.newSchemaSet(false)
		} else {
			//	notices = append(notices, *n)
			fmt.Printf("%s: %s\n", n.Severity, n.Message)
		}
		if cur > cnt {
			mig.Log.Warnf("Wrong tests count: test %d total %d", cur, cnt)
		}
	}
}

type fileDef struct {
	Name      string
	IfNewPkg  bool
	IfNewFile bool
}

type pkgDef struct {
	Name  string
	Op    string
	Files []fileDef
}

// Run does all work
func (mig *Migrator) Run(command string, packages []string) (*bool, error) {

	config, err := pgx.ParseEnvLibpq()
	// connStr := "postgres://@localhost/?sslmode=disable"
	// configMore, err := pgx.ParseURI(connStr)
	// config = config.Merge(configMore)
	if err != nil {
		return nil, errors.Wrap(err, "DB parse ENV")
	}

	config.OnNotice = mig.NoticeFunc()

	var files []pkgDef
	cfg := mig.Config
	empty := []string{}

	switch command {
	case "init":
		files, err = mig.lookupFiles(command, cfg.InitIncludes, cfg.NewIncludes, cfg.OnceIncludes, false, packages)
	case "test":
		files, err = mig.lookupFiles(command, cfg.TestIncludes, empty, empty, false, packages)
	case "drop":
		files, err = mig.lookupFiles(command, cfg.DropIncludes, empty, empty, true, packages)
	case "erase":
		files, err = mig.lookupFiles(command, cfg.EraseIncludes, empty, empty, true, packages)
	case "reinit":
		// drop, init
		files, err = mig.lookupFiles("drop", cfg.DropIncludes, empty, empty, true, packages)
		if err != nil {
			return nil, err
		}
		files1, err1 := mig.lookupFiles("init", cfg.InitIncludes, cfg.NewIncludes, cfg.OnceIncludes, false, packages)
		if err1 != nil {
			err = err1
		} else {
			files = append(files, files1...)
		}
	default:
		return nil, errors.New("Unknown command " + command)
	}
	if err != nil {
		return nil, err
	}
	var rv bool
	if len(files) == 0 {
		mig.Log.Warn("No files found")
		return &rv, nil
	}
	if cfg.ListOnly {
		// TODO: formatting
		fmt.Printf("Files:\n%#v\n", files)
		return &rv, nil
	}
	db, err := pgx.Connect(config)
	if err != nil {
		return nil, errors.Wrap(err, "DB connect")
	}

	tx, _ := db.Begin()
	defer tx.Rollback()

	err = mig.execFiles(tx, files)

	if err != nil && err != errCancel {
		return nil, err
	}
	if err != nil || mig.NoCommit() || mig.Config.NoCommit {
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

func (mig Migrator) lookupFiles(op string, masks []string, initMasks []string, onceMasks []string, isReverse bool, packages []string) (rv []pkgDef, err error) {
	pkgs := append(packages[:0:0], packages...)
	if isReverse {
		sort.Sort(sort.Reverse(sort.StringSlice(pkgs)))
	}
	for _, pkg := range pkgs {
		mig.Log.Debugf("Looking in %s for %v", pkg, masks)

		root := filepath.Join(mig.Config.Dir, pkg)
		var files []fileDef
		err = mig.FS.Walk(root, mig.WalkerFunc(masks, initMasks, onceMasks, &files))
		if err != nil {
			return rv, errors.Wrap(err, "Walk error")
		}
		if len(files) > 0 {
			mig.Log.Debugf("Found %d file(s)", len(files))
			sort.Slice(files, func(i, j int) bool {
				return files[i].Name < files[j].Name
			})
			rv = append(rv, pkgDef{Name: pkg, Op: op, Files: files})
		} else {
			mig.Log.Warnf("Package %s does not contain %v", pkg, masks)
		}
	}
	return
}

func (mig *Migrator) execFiles(tx *pgx.Tx, pkgs []pkgDef) error {

	for _, pkg := range pkgs {
		fmt.Printf("# %s.%s\n", pkg.Name, pkg.Op)

		if !mig.Config.NoHooks {
			// TODO: skip on pgmig init
			//_, err = tx.Exec(mig.Config.HookBefore, pkg.Op, pkg.Name)
		}
		mig.newSchemaSet(true)
		for _, file := range pkg.Files {
			fmt.Printf("\t%s\n", file.Name)
			if file.IfNewPkg {
				mig.Log.Debugf("only new: %s\n", file.Name)
				if !mig.newSchema() {
					mig.Log.Debugf("Skip file %s:%s because pkg is old", pkg.Name, file.Name)
					continue
				}
			}
			if file.IfNewFile {
				isNew := false // TODO: ask DB
				if !isNew {
					mig.Log.Debugf("Skip file %s:%s because it is loaded already", pkg.Name, file.Name)
					// TODO: check csum
					continue
				}
			}

			f := filepath.Join(mig.Config.Dir, pkg.Name, file.Name)
			s, err := ioutil.ReadFile(f)
			if err != nil {
				return errors.Wrap(err, "Reading "+f)
			}
			query := string(s)
			//fmt.Print(query)
			_, err = tx.Exec(query)
			if err == nil {
				// NOTICE>>: schema "pgmig" already exists, skipping
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
				fmt.Printf(">>%s:%d [%s %s] %s\n", file.Name, lineNo, pgErr.Severity, pgErr.Code, pgErr.Message)
				if pgErr.Detail != "" {
					fmt.Printf("DETAIL: %s\n", pgErr.Detail)
				}
				if pgErr.Hint != "" {
					fmt.Printf("HINT: %s\n", pgErr.Hint)
				}
			}
			return errCancel
		}
		if !mig.Config.NoHooks {
			// skip if pgmig.erase
			// _, err = tx.Exec(mig.Config.HookAfter, pkg.Op, pkg.Name)
		}

	}
	return nil
}

func (mig Migrator) WalkerFunc(mask []string, initMasks []string, onceMasks []string, files *[]fileDef) func(path string, f os.FileInfo, err error) error {
	return func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if f.IsDir() {
			return nil
		}
		var matched bool
		for _, m := range mask {
			if m[0] == byte(33) { // "!"
				matchedExclude, err := filepath.Match(m[1:], f.Name())
				if err != nil {
					return err
				}
				if matchedExclude {
					return nil
				}
			} else if !matched {
				matched, err = filepath.Match(m, f.Name())
				if err != nil {
					return err
				}
			}
		}
		if !matched {
			return nil
		}

		def := fileDef{Name: f.Name()}
		for _, m := range initMasks {
			matched, err = filepath.Match(m, f.Name())
			if err != nil {
				return err
			}
			if matched {
				def.IfNewPkg = true
				break
			}
		}
		for _, m := range onceMasks {
			matched, err = filepath.Match(m, f.Name())
			if err != nil {
				return err
			}
			if matched {
				def.IfNewFile = true
				break
			}
		}
		*files = append(*files, def)
		return nil
	}
}

func (mig *Migrator) newSchemaSet(isNew bool) {
	mig.isNewLock.Lock()
	defer mig.isNewLock.Unlock()
	mig.isNew = isNew
}

func (mig *Migrator) newSchema() bool {
	mig.isNewLock.RLock()
	defer mig.isNewLock.RUnlock()
	return mig.isNew
}
