package pgmig

import (
	//"os"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"

	mapper "github.com/birkirb/loggers-mapper-logrus"
	"github.com/jackc/pgconn"
	"github.com/sirupsen/logrus/hooks/test"
)

func ExamplePrintMessages() {

	l, _ := test.NewNullLogger()
	log := mapper.NewLogger(l)
	mig := New(Config{}, log, nil, "")
	mig.IsTerminal = false
	mig.MessageChan = make(chan interface{}, 50)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go mig.PrintMessages(&wg)

	tests := []interface{}{
		&Status{Exists: true},
		//		mig.MessageChan <- pgErr
		&Op{Pkg: "pkg.Name", Op: "pkg.Op"},
		&Version{Version: "installedVersion"},
		&NewVersion{Version: "info.Version", Repo: "info.Repository"},
		&RunFile{Name: "file.Name"},
		&TestCount{Count: 1},
		&TestOk{Current: 1, Message: "message"},
		&TestFail{Current: 1, Message: "message", Detail: "detail"},
		&pgconn.PgError{
			File:          "file",
			Line:          1,
			Severity:      "NOTICE",
			Code:          "0000",
			Message:       "msg",
			Detail:        "det",
			Hint:          "hint",
			Where:         "where",
			InternalQuery: "query",
		},
	}
	for _, tt := range tests {
		mig.MessageChan <- tt
	}
	close(mig.MessageChan)
	wg.Wait()
	// Output:
	// PgMig exists: true
	// # pkg.Name.pkg.Op
	// Installed version: installedVersion
	// New version:       info.Version from info.Repository
	//
	// # file.Name
	// 1..1
	// ok 1 - message
	// not ok 1 - message
	//   ---
	// detail
	//   ---
	// #  file:1 NOTICE 0000 msg
	// #  Detail: det
	// #  Hint: hint
	// #  Where: where
	// #  Query: query

}

func TestColors(t *testing.T) {

	a, b, c, d := colors(true)
	got := []string{a, b, c, d}
	assert.Equal(t, []string{"\x1b[33m", "\x1b[32m", "\x1b[31m", "\x1b[m"}, got)
}
