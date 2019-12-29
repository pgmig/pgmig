package pgmig

import (
	"fmt"
	"sync"

	"github.com/jackc/pgconn"
)

type Status struct {
	Exists bool
}

//	type pgErr
type Op struct {
	Pkg string
	Op  string
}
type Version struct{ Version string }
type NewVersion struct {
	Version string
	Repo    string
}
type RunFile struct{ Name string }
type TestCount struct{ Count int }
type TestOk struct {
	Current int
	Message string
}
type TestFail struct {
	Current int
	Message string
	Detail  string
}

func (mig *Migrator) PrintMessages(wg *sync.WaitGroup) {

	yellow, green, red, end := colors(mig.IsTerminal)
	for m := range mig.MessageChan {

		switch v := m.(type) {
		case *Status:
			fmt.Printf("PgMig exists: %v\n", v.Exists)
		case *Op:
			fmt.Printf("%s# %s.%s%s\n", yellow, v.Pkg, v.Op, end)
		case *Version:
			fmt.Printf("Installed version: %s\n", v.Version)
		case *NewVersion:
			fmt.Printf("New version:       %s from %s\n", v.Version, v.Repo)
		case *RunFile:
			if mig.IsTerminal {
				fmt.Printf("\r# %s ", v.Name)
			} else {
				fmt.Printf("\n# %s", v.Name)
			}
		case *TestCount:
			fmt.Printf("\n%d..%d\n", 1, v.Count)
		case *TestOk:
			fmt.Printf("%sok %d - %s%s\n", green, v.Current, v.Message, end)
		case *TestFail:
			fmt.Printf("%snot ok %d - %s\n  ---\n%s%s\n  ---\n", red, v.Current, v.Message, v.Detail, end)
		case *pgconn.PgError:
			printPgError(v)
		default:
			fmt.Printf(">> %T\n", m)
		}
	}
	mig.Log.Debug("MessageChan closed")
	wg.Done()
}

func colors(isTerm bool) (string, string, string, string) {
	if isTerm {
		return "\033[33m", "\033[32m", "\033[31m", "\033[m"
	}
	return "", "", "", ""
}

// printPgError print Pg error struct
func printPgError(e *pgconn.PgError) {
	fmt.Printf("#  %s:%d %s %s %s\n", e.File, e.Line, e.Severity, e.Code, e.Message)
	if e.Detail != "" {
		fmt.Println("#  Detail: " + e.Detail)
	}
	if e.Hint != "" {
		fmt.Println("#  Hint: " + e.Hint)
	}
	if e.Where != "" {
		fmt.Println("#  Where: " + e.Where)
	}
	if e.InternalQuery != "" {
		fmt.Println("#  Query: " + e.InternalQuery)
	}
}
