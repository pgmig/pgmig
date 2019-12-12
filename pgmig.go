package pgmig

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/pkg/errors"
	"gopkg.in/birkirb/loggers.v1"
)

// Config holds all config vars
type Config struct {
	Dir      string `long:"dir" default:"sql" description:"SQL sources directory"` // TODO: pkg/*/sql
	NoCommit bool   `long:"nocommit" description:"Do not commit work"`
	ListOnly bool   `long:"listonly" description:"Show file list and exit"`
	Debug    bool   `long:"debug" description:"Print debug info"` // TODO: process

	// TODO: SearchPath?

	// TODO: Force    bool   `long:"force" description:"Allow erase command"`
	NoHooks    bool   `long:"nohooks" description:"Do not call before/after hooks"`
	HookBefore string `long:"hook_before" default:"pkg_op_before" description:"Func called before command for every pkg"`
	HookAfter  string `long:"hook_after" default:"pkg_op_after" description:"Func called after command for every pkg"`

	PkgVersion string `long:"pkg_version" default:"pkg_version" description:"Func for fetching installed package version"`

	ScriptProtected string `long:"script_protected" default:"script_protected" description:"Func for fetchng md5 of protected script"`
	ScriptProtect   string `long:"script_protect" default:"script_protect" description:"Func for saving md5 of protected script"`

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
	ReadFile(name string) (string, error)
}

// Migrator holds service data
type Migrator struct {
	Config     *Config
	Log        loggers.Contextual
	FS         FileSystem
	doRollback bool
	installed  bool
	commitLock sync.RWMutex
	cur        int
	cnt        int
}

const (
	// CmdInit holds name of init command
	CmdInit = "init"
	// CmdTest holds name of test command
	CmdTest = "test"
	// CmdDrop  holds name of drop command
	CmdDrop = "drop"
	// CmdErase holds name of erase command
	CmdErase = "erase"
	// CmdReInit holds name of reinit (drop+init) command
	CmdReInit = "reinit"
	// CmdList holds name of list command
	CmdList = "list" // TODO

	// CorePackage is the name of pgmig core package
	CorePackage = "pgmig"
	// CoreTable is the name of table inside core package(scheme) which must exist if pgmig is installed already
	CoreTable = "pkg"

	pgStatusTestCount = "01998"
	pgStatusTestOk    = "01999"
	pgStatusTestFail  = "02999"

	// SQLPgMigExists is a sql query to check pgmig.pkg table presense
	SQLPgMigExists = "SELECT true FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2"
	// SQLPkgVersion is a query for installed package version
	SQLPkgVersion      = "SELECT %s.%s($1)"
	SQLPkgOpBefore     = "SELECT %s.%s(a_op => $1, a_code => $2, a_version => $3, a_repo => $4)"
	SQLPkgOpAfter      = "SELECT %s.%s(a_op => $1, a_code => $2, a_version => $3, a_repo => $4)"
	SQLScriptProtected = "SELECT %s.%s(a_pkg => $1, a_file => $2)"
	SQLScriptProtect   = "SELECT %s.%s(a_pkg => $1, a_file => $2, a_md5 => $3)"
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
func (mig *Migrator) Run(tx pgx.Tx, command string, packages []string) (*bool, error) {

	var files []pkgDef
	cfg := mig.Config
	empty := []string{}
	var err error
	var rv bool

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
			return &rv, nil
		}
		files1, err1 := mig.lookupFiles(CmdInit, cfg.InitIncludes, cfg.NewIncludes, cfg.OnceIncludes, false, packages)
		if err1 != nil {
			err = err1
		} else {
			files = append(files, files1...)
		}
	default:
		return &rv, errors.New("Unknown command " + command)
	}
	if err != nil {
		return &rv, nil
	}
	if len(files) == 0 {
		mig.Log.Warn("No files found")
		return &rv, nil
	}
	if cfg.ListOnly {
		// TODO: formatting
		fmt.Printf("Files:\n%#v\n", files)
		return &rv, nil
	}

	err = queryValue(tx, &mig.installed, SQLPgMigExists, CorePackage, CoreTable)
	if err != nil {
		return &rv, errors.Wrap(err, "Check pgmig")
	}

	fmt.Printf("PgMig exists: %v\n", mig.installed)

	err = mig.execFiles(tx, files)
	if err != nil {
		pgErr, ok := err.(*pgconn.PgError)
		if !ok {
			return &rv, errors.Wrap(err, "System error")
		}
		mig.Log.Debugf("%s:%d\n"+pgErr.Hint, pgErr.File, pgErr.Line)
		mig.Log.Debugf("\n(%#v)", pgErr)
		if pgErr.Where != "" {
			mig.Log.Debug("\n" + pgErr.Where)
		}
		if pgErr.InternalQuery != "" {
			mig.Log.Debug("\n" + pgErr.InternalQuery)
		}
		return &rv, errors.Wrap(pgErr, "DB error")
	}
	if mig.noCommit() || mig.Config.NoCommit || command == CmdTest {
		rv = false
	} else {
		rv = true
	}
	if err != nil {
		return &rv, errors.Wrap(err, "End work error")
	}
	return &rv, nil
}

func (mig *Migrator) execFiles(tx pgx.Tx, pkgs []pkgDef) error {
	for _, pkg := range pkgs {
		fmt.Printf("# %s.%s\n", pkg.Name, pkg.Op)
		var installedVersion string
		if mig.installed {
			err := queryValue(tx, &installedVersion, fmt.Sprintf(SQLPkgVersion, CorePackage, mig.Config.PkgVersion), pkg.Name)
			if err != nil {
				return err
			}
			if installedVersion != "" {
				fmt.Printf("Installed version: %s\n", installedVersion)
			}
		}
		pkgExists := (installedVersion != "")
		ctx := context.Background()
		var version, repo string
		if !mig.Config.NoHooks && pkg.Op != CmdTest {
			// hooks enabled
			if pkg.Op == CmdInit {
				if err := GitVersion(pkg.Root, &version); err != nil {
					return err
				}
				if err := GitRepo(pkg.Root, &repo); err != nil {
					return err
				}
				fmt.Printf("New version:       %s from %s\n", version, repo)
			}
			if !(pkg.Name == CorePackage && pkg.Op == CmdInit && !pkgExists) {
				// this is not "init" for new CorePackage
				if _, err := tx.Exec(ctx, fmt.Sprintf(SQLPkgOpBefore, CorePackage, mig.Config.HookBefore),
					pkg.Op, pkg.Name, version, repo); err != nil {
					return err
				}
			}
		}
		for _, file := range pkg.Files {
			fmt.Printf("\t%s\n", file.Name)
			if file.IfNewPkg {
				if pkgExists {
					mig.Log.Debugf("Skip file %s/%s because pkg is old", pkg.Name, file.Name)
					continue
				}
			}
			f := filepath.Join(pkg.Root, file.Name)
			s, err := mig.FS.ReadFile(f)
			if err != nil {
				return errors.Wrap(err, "Reading "+f)
			}

			if file.IfNewFile {
				var md5Old *string
				err := queryValue(tx, &md5Old, fmt.Sprintf(SQLScriptProtected, CorePackage, mig.Config.ScriptProtected),
					pkg.Name, file.Name)
				if err != nil {
					return errors.Wrap(err, "SQLScriptProtected")
				}
				md5New := fmt.Sprintf("%x", md5.Sum([]byte(s)))
				if md5Old != nil {
					mig.Log.Debugf("Skip file %s/%s because it is loaded already", pkg.Name, file.Name)
					fmt.Printf("Skip file %s/%s because it is loaded already", pkg.Name, file.Name)
					if *md5Old != md5New {
						mig.Log.Warnf("Warning md5 changed for %s:%s from %s to %s", pkg.Name, file.Name, *md5Old, md5New)
					}
					continue
				}
				_, err = tx.Exec(ctx, fmt.Sprintf(SQLScriptProtect, CorePackage, mig.Config.ScriptProtect),
					pkg.Name, file.Name, md5New)
				if err != nil {
					return errors.Wrap(err, "SQLScriptProtect")
				}
			}

			query := string(s)
			//fmt.Print(">>>>>>>>>", query)
			_, err = tx.Exec(ctx, query)
			if err != nil {
				pgErr, ok := err.(*pgconn.PgError)
				if !ok {
					return errors.Wrap(err, "System error")
				}
				fmt.Printf(">>>> %#v", pgErr)
				pgErr.File = file.Name
				pgErr.Line = int32(strings.Count(string([]rune(query)[:pgErr.Position]), "\n") + 1)
				return pgErr
			}
		}
		if !mig.Config.NoHooks && pkg.Op != CmdTest {
			if pkg.Name == CorePackage && (pkg.Op == CmdDrop || pkg.Op == CmdErase) {
				mig.installed = false
				mig.Log.Debug("pgmig is not installed now")
			} else {
				// hooks enabled and this is not drop/erase for CorePackage
				if _, err := tx.Exec(ctx, fmt.Sprintf(SQLPkgOpAfter, CorePackage, mig.Config.HookAfter),
					pkg.Op, pkg.Name, version, repo); err != nil {
					fmt.Printf(">>>> %#v", err)
					return errors.Wrap(err, "SQLPkgOpAfter")
				}
			}
		}
	}
	return nil
}

// TODO: get some from embedded FS, other form subdir
func (mig *Migrator) lookupFiles(op string, masks []string, initMasks []string, onceMasks []string, isReverse bool, packages []string) (rv []pkgDef, err error) {
	pkgs := append(packages[:0:0], packages...) // Copy slice. See https://github.com/go101/go101/wiki
	if isReverse {
		SliceReverse(pkgs)
		mig.Log.Debugf("Packages: %#v", pkgs)
	}
	for _, pkg := range pkgs {
		mig.Log.Debugf("Looking in %s for %v", pkg, masks)

		root := filepath.Join(mig.Config.Dir, pkg)
		var files []fileDef
		err = mig.FS.Walk(root, mig.walkerFunc(masks, initMasks, onceMasks, &files))
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

// ProcessNotice receives PG notices with test metadata and plain
// TODO: add multiprocess support?
func (mig *Migrator) ProcessNotice(code, message, detail string) {
	switch code {
	case pgStatusTestCount:
		mig.cnt, _ = strconv.Atoi(message)
		mig.cur = 0
		//			notices = []pgx.Notice{}
	case pgStatusTestOk:
		mig.cur++
		fmt.Printf("\t\t(%d/%d) %-20s: Ok\n", mig.cur, mig.cnt, message)
		//			notices = []pgx.Notice{}
	case pgStatusTestFail:
		mig.cur++
		// TODO: send to channel {Type:.., Message: []string}
		fmt.Printf("\t\t(%d/%d) %-20s: Not Ok\n%s\n", mig.cur, mig.cnt, message, detail)
		//			if len(notices) > 0 {
		//				fmt.Println(notices)
		//			}
		//			notices = []pgx.Notice{}
		mig.setNoCommit(true)
	default:
		//	notices = append(notices, *n)
		//mig.Log.Infof("%s: %s\n", n.Severity, n.Message)
	}
	if mig.cur > mig.cnt && (code == pgStatusTestOk || code == pgStatusTestFail) {
		mig.Log.Warnf("Wrong tests count: test %d total %d", mig.cur, mig.cnt)
	}
}

// walkerFunc walks throush filesystem and return list of files to run
func (mig *Migrator) walkerFunc(mask []string, initMasks []string, onceMasks []string, files *[]fileDef) func(path string, f os.FileInfo, err error) error {
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

// setNoCommit sets commit status
func (mig *Migrator) setNoCommit(doRollback bool) {
	mig.commitLock.Lock()
	defer mig.commitLock.Unlock()
	mig.doRollback = doRollback
}

// noCommit returns commit status
func (mig *Migrator) noCommit() bool {
	mig.commitLock.RLock()
	defer mig.commitLock.RUnlock()
	return mig.doRollback
}

// queryValue fills rv with single valued SQL result if present
func queryValue(tx pgx.Tx, rv interface{}, sql string, arguments ...interface{}) error {
	rows, err := tx.Query(context.Background(), sql, arguments...)
	defer rows.Close()
	if err != nil {
		return err
	}
	if rows.Next() {
		err = rows.Scan(rv)
		if err != nil {
			return errors.Wrap(err, "Incompartible value returned")
		}
	}
	return nil
}

// SliceReverse replace the contents of a slice with the same elements but in reverse order
// See https://github.com/golang/go/wiki/SliceTricks#reversing
func SliceReverse(pkgs []string) {
	for i := len(pkgs)/2 - 1; i >= 0; i-- {
		opp := len(pkgs) - 1 - i
		pkgs[i], pkgs[opp] = pkgs[opp], pkgs[i]
	}
}
