package pgmig

import (
	//	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
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
	HookBefore string `long:"hook_before" default:"pkg_op_before" description:"Func called before command for every pkg"`
	HookAfter  string `long:"hook_after" default:"pkg_op_after" description:"Func called after command for every pkg"`

	//nolint:staticcheck // Multiple struct tag "default" is allowed
	InitIncludes []string `long:"init" default:"*.sql" default:"!*.drop.sql" default:"!*.erase.sql" description:"File masks for init command"`
	TestIncludes []string `long:"test" default:"*.test.sql" description:"File masks for test command"`
	NewIncludes  []string `long:"new" default:"*.new.sql" description:"File masks loaded on init if package is new"`
	DropIncludes []string `long:"drop" default:"*.drop.sql" description:"File masks for drop command"`
	//nolint:staticcheck // Multiple struct tag "default" is allowed
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
	Config   *Config
	Log      loggers.Contextual
	FS       FileSystem
	noCommit bool
	//isNew      bool
	commitLock sync.RWMutex
}

const (
	CmdInit   = "init"
	CmdTest   = "test"
	CmdDrop   = "drop"
	CmdErase  = "erase"
	CmdReInit = "reinit"
	CmdList   = "list" // TODO

	CorePackage = "pgmig"

	pgStatusTestCount = "01998"
	pgStatusTestOk    = "01999"
	pgStatusTestFail  = "02999"

	SQLPkgExists   = "SELECT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname = $1)"
	SQLPkgOpBefore = "SELECT %s.%s(a_op => $1, a_code=> $2, a_version => $3, a_repo => $4)"
	SQLPkgOpAfter  = "SELECT %s.%s(a_op => $1, a_code=> $2, a_version => $3, a_repo => $4)"
)

// New creates an Migrator object
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

type fileDef struct {
	Name      string
	IfNewPkg  bool
	IfNewFile bool
}

type pkgDef struct {
	Name  string
	Op    string
	Root  string
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
	case CmdInit:
		files, err = mig.lookupFiles(command, cfg.InitIncludes, cfg.NewIncludes, cfg.OnceIncludes, false, packages)
	case CmdTest:
		files, err = mig.lookupFiles(command, cfg.TestIncludes, empty, empty, false, packages)
	case CmdDrop:
		files, err = mig.lookupFiles(command, cfg.DropIncludes, empty, empty, true, packages)
	case CmdErase:
		files, err = mig.lookupFiles(command, cfg.EraseIncludes, empty, empty, true, packages)
	case CmdReInit:
		// drop, init
		files, err = mig.lookupFiles(CmdDrop, cfg.DropIncludes, empty, empty, true, packages)
		if err != nil {
			return nil, err
		}
		files1, err1 := mig.lookupFiles(CmdInit, cfg.InitIncludes, cfg.NewIncludes, cfg.OnceIncludes, false, packages)
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
	defer func() {
		err = tx.Rollback()
		if err != nil && err != pgx.ErrTxClosed {
			mig.Log.Error(err)
		}
	}()
	err = mig.execFiles(tx, files)
	if err != nil {
		pgErr, ok := err.(pgx.PgError)
		if !ok {
			return nil, errors.Wrap(err, "System error")
		}
		mig.Log.Debugf("%s:%d\n"+pgErr.Hint, pgErr.File, pgErr.Line)
		mig.Log.Debugf("\n(%#v)", pgErr)
		if pgErr.Where != "" {
			mig.Log.Debugf("\n" + pgErr.Where)
		}
		if pgErr.InternalQuery != "" {
			mig.Log.Debugf("\n" + pgErr.InternalQuery)
		}
		return nil, errors.Wrap(err, "DB error")
	}
	if mig.NoCommit() || mig.Config.NoCommit || command == CmdTest {
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

func (mig *Migrator) execFiles(tx *pgx.Tx, pkgs []pkgDef) error {

	for _, pkg := range pkgs {
		fmt.Printf("# %s.%s\n", pkg.Name, pkg.Op)

		var schemaExists bool
		err := tx.QueryRow(SQLPkgExists, pkg.Name).Scan(&schemaExists)
		if err != nil {
			return err
		}
		// TODO: if schemaExists { get & print repo:version }
		mig.Log.Debugf("Start package: %v / %s / %s / %v", mig.Config.NoHooks, pkg.Name, pkg.Op, schemaExists)

		var version, repo string
		if !mig.Config.NoHooks && pkg.Op != CmdTest {
			// hooks enabled
			if pkg.Op == CmdInit {
				err = PkgVersion(pkg.Root, &version)
				if err != nil {
					return err
				}
				err = PkgRepo(pkg.Root, &repo)
				if err != nil {
					return err
				}
				fmt.Printf("Meta:  %s\t%s\n", version, repo)
			}
			if !(pkg.Name == CorePackage && pkg.Op == CmdInit && !schemaExists) {
				// this is not init for new CorePackage
				_, err = tx.Exec(fmt.Sprintf(SQLPkgOpBefore, CorePackage, mig.Config.HookBefore), pkg.Op, pkg.Name, version, repo)
				if err != nil {
					return err
				}
			}
		}
		for _, file := range pkg.Files {
			fmt.Printf("\t%s\n", file.Name)
			if file.IfNewPkg {
				mig.Log.Debugf("only new: %s\n", file.Name)
				if schemaExists {
					mig.Log.Debugf("Skip file %s:%s because pkg is old", pkg.Name, file.Name)
					continue
				}
			}
			if file.IfNewFile {
				/*
				   TODO:
				   * calc md5
				   * isNewFile, err = tx.SelectValue(mig.Config.IsNewFile, pkg.Op, pkg.Name, file.Name, md5)
				*/
				isNewFile := false // TODO: ask DB
				if !isNewFile {
					mig.Log.Debugf("Skip file %s:%s because it is loaded already", pkg.Name, file.Name)
					// TODO: check csum
					continue
				}
			}

			f := filepath.Join(pkg.Root, file.Name)
			s, err := ioutil.ReadFile(f)
			if err != nil {
				return errors.Wrap(err, "Reading "+f)
			}
			query := string(s)
			//fmt.Print(query)
			_, err = tx.Exec(query)
			if err != nil {
				pgErr, ok := err.(pgx.PgError)
				if !ok {
					return errors.Wrap(err, "System error")
				}
				pgErr.File = file.Name
				pgErr.Line = int32(strings.Count(string([]rune(query)[:pgErr.Position]), "\n") + 1)
				return pgErr
			}
		}
		if !mig.Config.NoHooks &&
			pkg.Op != CmdTest &&
			!(pkg.Name == CorePackage && (pkg.Op == CmdDrop || pkg.Op == CmdErase)) {
			// hooks enabled and this is not drop/erase for CorePackage
			_, err := tx.Exec(fmt.Sprintf(SQLPkgOpAfter, CorePackage, mig.Config.HookAfter), pkg.Op, pkg.Name, version, repo)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

// TODO: get some from embedded FS, other form subdir
func (mig *Migrator) lookupFiles(op string, masks []string, initMasks []string, onceMasks []string, isReverse bool, packages []string) (rv []pkgDef, err error) {
	pkgs := append(packages[:0:0], packages...) // Copy slice. See https://github.com/go101/go101/wiki
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
			rv = append(rv, pkgDef{Name: pkg, Op: op, Root: root, Files: files})
		} else {
			mig.Log.Warnf("Package %s does not contain %v", pkg, masks)
		}
	}
	return
}

// NoticeFunc receives PG notices with test metadata and plain
func (mig *Migrator) NoticeFunc() func(c *pgx.Conn, n *pgx.Notice) {
	var cnt int
	var cur int
	return func(c *pgx.Conn, n *pgx.Notice) {
		if n.Code == pgStatusTestCount {
			cnt, _ = strconv.Atoi(n.Message)
			cur = 0
			//			notices = []pgx.Notice{}
		} else if n.Code == pgStatusTestOk {
			cur++
			fmt.Printf("(%d/%d) %-20s: Ok\n", cur, cnt, n.Message)
			//			notices = []pgx.Notice{}
		} else if n.Code == pgStatusTestFail {
			cur++
			fmt.Printf("(%d/%d) %-20s: Not Ok\n%s\n", cur, cnt, n.Message, n.Detail)
			//			if len(notices) > 0 {
			//				fmt.Println(notices)
			//			}
			//			notices = []pgx.Notice{}
			mig.SetNoCommit(true)
		} else {
			//	notices = append(notices, *n)
			fmt.Printf("%s: %s\n", n.Severity, n.Message)
		}
		if cur > cnt && (n.Code == pgStatusTestOk || n.Code == pgStatusTestFail) {
			mig.Log.Warnf("Wrong tests count: test %d total %d", cur, cnt)
		}
	}
}
func (mig *Migrator) WalkerFunc(mask []string, initMasks []string, onceMasks []string, files *[]fileDef) func(path string, f os.FileInfo, err error) error {
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

func (mig *Migrator) SetNoCommit(commit bool) {
	mig.commitLock.Lock()
	defer mig.commitLock.Unlock()
	mig.noCommit = commit
}

func (mig *Migrator) NoCommit() bool {
	mig.commitLock.RLock()
	defer mig.commitLock.RUnlock()
	return mig.noCommit
}

func PkgVersion(path string, rv *string) error {
	out, err := exec.Command("git", "-C", path, "describe", "--tags", "--always").Output()
	if err != nil {
		return err
	}
	*rv = strings.TrimSuffix(string(out), "\n")
	return nil
}

func PkgRepo(path string, rv *string) error {
	out, err := exec.Command("git", "-C", path, "config", "--get", "remote.origin.url").Output()
	if err != nil {
		return err
	}
	*rv = strings.TrimSuffix(string(out), "\n")
	return nil
}
