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
	ListOnly bool   `long:"listonly" description:"Show file list and exit"`
	// TODO: SearchPath

	CreateInclude []string `long:"create" default:"*.sql" default:"!*.drop.sql" default:"!*.clean.sql" description:"File masks for create command"`
	TestInclude   []string `long:"test" default:"*.test.sql" description:"File masks for test command"`
	CleanInclude  []string `long:"clean" default:"*.clean.sql" description:"File masks for clean command"`
	DropInclude   []string `long:"drop" default:"*.drop.sql" default:"*.clean.sql" description:"File masks for drop command"`
	InitInclude   []string `long:"init" default:"*.init.sql" description:"File masks loaded on create if package is new"`
	OnceInclude   []string `long:"once" default:"*.once.sql" description:"File masks loaded once on create"`
}

// FileSystem holds all of used filesystem access methods
type FileSystem interface {
	Walk(root string, walkFn filepath.WalkFunc) error
	Open(name string) (http.File, error)
}

// Migrator holds service data
type Migrator struct {
	Config   *Config
	Log      loggers.Contextual
	FS       FileSystem
	noCommit bool
}

var errCancel = errors.New("Rollback")

const (
	pgStatusTestCount = "01998"
	pgStatusTestOk    = "01999"
	pgStatusTestFail  = "02999"
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
		} else {
			//	notices = append(notices, *n)
			fmt.Printf("%s: %s\n", n.Severity, n.Message)
		}
	}
}

type fileDef struct {
	Pkg       string
	Name      string
	IfNewPkg  bool
	IfNewFile bool
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

	var files []fileDef
	cfg := mig.Config
	empty := []string{}

	switch command {
	case "create":
		files, err = mig.lookupFiles(cfg.CreateInclude, cfg.InitInclude, cfg.OnceInclude, false, packages)
	case "test":
		files, err = mig.lookupFiles(cfg.TestInclude, empty, empty, false, packages)
	case "clean":
		files, err = mig.lookupFiles(cfg.CleanInclude, empty, empty, true, packages)
	case "drop":
		files, err = mig.lookupFiles(cfg.DropInclude, empty, empty, true, packages)
	case "recreate":
		// clean, create
		files, err = mig.lookupFiles(cfg.CleanInclude, empty, empty, true, packages)
		if err != nil {
			return nil, err
		}
		files1, err1 := mig.lookupFiles(cfg.CreateInclude, cfg.InitInclude, cfg.OnceInclude, false, packages)
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

func (mig Migrator) lookupFiles(masks []string, initMasks []string, onceMasks []string, isReverse bool, packages []string) (rv []fileDef, err error) {
	pkgs := append(packages[:0:0], packages...)
	if isReverse {
		sort.Sort(sort.Reverse(sort.StringSlice(pkgs)))
	}
	for _, pkg := range pkgs {
		mig.Log.Debugf("Looking in %s for %v", pkg, masks)

		root := filepath.Join(mig.Config.Dir, pkg)
		var files []fileDef
		err = mig.FS.Walk(root, mig.WalkerFunc(pkg, masks, initMasks, onceMasks, &files))
		if err != nil {
			return rv, errors.Wrap(err, "Walk error")
		}
		if len(files) > 0 {
			mig.Log.Debugf("Found %d file(s)", len(files))
			sort.Slice(files, func(i, j int) bool {
				return files[i].Name < files[j].Name
			})
			rv = append(rv, files...)
		} else {
			mig.Log.Warnf("Package %s does not contain %v", pkg, masks)
		}
	}
	return
}

func (mig Migrator) execFiles(tx *pgx.Tx, files []fileDef) error {

	newPkgs := map[string]bool{} // cache for packages state
	for _, file := range files {
		fmt.Printf("Load %s:%s\n", file.Pkg, file.Name)

		if file.IfNewPkg {
			isNew, ok := newPkgs[file.Pkg]
			if !ok {
				isNew = true //  TODO: ask DB
				newPkgs[file.Pkg] = isNew
			}
			if !isNew {
				mig.Log.Debugf("Skip file %s:%s because pkg is old", file.Pkg, file.Name)
				continue
			}
		}
		if file.IfNewFile {
			isNew := false // TODO: ask DB
			if !isNew {
				mig.Log.Debugf("Skip file %s:%s because it is loaded already", file.Pkg, file.Name)
				// TODO: check csum
				continue
			}
		}

		f := filepath.Join(mig.Config.Dir, file.Pkg, file.Name)
		s, err := ioutil.ReadFile(f)
		if err != nil {
			return errors.Wrap(err, "Reading "+f)
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
	return nil
}

func (mig Migrator) WalkerFunc(pkg string, mask []string, initMasks []string, onceMasks []string, files *[]fileDef) func(path string, f os.FileInfo, err error) error {
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

		def := fileDef{Pkg: pkg, Name: f.Name()}
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
