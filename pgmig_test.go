//go:generate mockgen -destination=generated_mock_test.go -package pgmig github.com/jackc/pgx/v4 Tx,Rows

package pgmig

import (
	"context"
	"crypto/md5"
	"fmt"
	"github.com/golang/mock/gomock"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	mapper "github.com/birkirb/loggers-mapper-logrus"
	"github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"

	"github.com/jackc/pgconn"
)

type ServerSuite struct {
	suite.Suite
	cfg  Config
	srv  *Migrator
	hook *test.Hook
}

var (
	version = "loaded from git in SetupSuite"
)

func (ss *ServerSuite) SetupSuite() {

	// Fill config with default values
	p := flags.NewParser(&ss.cfg, flags.Default|flags.IgnoreUnknown)
	_, err := p.Parse()
	require.NoError(ss.T(), err)

	l, hook := test.NewNullLogger()
	ss.hook = hook
	l.SetLevel(logrus.DebugLevel)
	log := mapper.NewLogger(l)

	hook.Reset()

	ctrl := gomock.NewController(ss.T())
	defer ctrl.Finish()

	ss.srv = New(ss.cfg, log, defaultFS{}, "testdata")
}

func TestSuite(t *testing.T) {

	myTest := &ServerSuite{}
	suite.Run(t, myTest)

}

func (ss *ServerSuite) TestRun() {
	ctx := context.Background()

	ctrl := gomock.NewController(ss.T())
	defer ctrl.Finish()
	tx := NewMockTx(ctrl)

	rv := NewMockRows(ctrl)
	rv.EXPECT().Next().Return(false)
	rv.EXPECT().Close()
	rv.EXPECT().Next()
	rv.EXPECT().Close()

	ct0 := pgconn.CommandTag{} // {0, ' ', 0}
	//fmt.Println(ct0)
	mig := ss.srv

	cf := func(file string) string {
		return string(content(ss.T(), mig, file))
	}
	ex := tx.EXPECT()
	gomock.InOrder(
		ex.Query(ctx, SQLPgMigExists, "pgmig", "pkg").Return(rv, nil),
		ex.Exec(ctx, fmt.Sprintf(SQLPkgOp, CorePackage, mig.Config.HookBefore), "init", "a", version, "git@github.com:pgmig/pgmig.git").Return(ct0, nil),
		ex.Exec(ctx, cf("a/00_init.sql")),
		ex.Exec(ctx, cf("a/01_ddl.sql")),
		ex.Exec(ctx, cf("a/02_ddl.test.sql")),
		ex.Query(ctx, fmt.Sprintf(SQLScriptProtected, CorePackage, mig.Config.ScriptProtected), "a", "03.once.sql").Return(rv, nil),
		ex.Exec(ctx, fmt.Sprintf(SQLScriptProtect, CorePackage, mig.Config.ScriptProtect), "a", "03.once.sql", fmt.Sprintf("%x", md5.Sum(content(ss.T(), mig, "a/03.once.sql")))),
		ex.Exec(ctx, cf("a/03.once.sql")),
		ex.Exec(ctx, cf("a/04.new.sql")),
		ex.Exec(ctx, fmt.Sprintf(SQLPkgOp, CorePackage, mig.Config.HookAfter), "init", "a", version, "git@github.com:pgmig/pgmig.git"),
	)
	mig.MessageChan = make(chan interface{}, 8)
	commit, err := mig.Run(tx, "init", []string{"a"}) // []string{"a", "b"})
	close(mig.MessageChan)
	//	ss.printLogs()
	assert.Nil(ss.T(), err)
	assert.Equal(ss.T(), *commit, true)
	got := []interface{}{}
	for s := range mig.MessageChan {
		got = append(got, s)
	}
	want := []interface{}{
		&Status{Exists: false},
		&Op{Pkg: "a", Op: "init"},
		&NewVersion{Version: "loaded from git in SetupSuite", Repo: "git@github.com:pgmig/pgmig.git"},
		&RunFile{Name: "00_init.sql"},
		&RunFile{Name: "01_ddl.sql"},
		&RunFile{Name: "02_ddl.test.sql"},
		&RunFile{Name: "03.once.sql"},
		&RunFile{Name: "04.new.sql"},
	}
	assert.Equal(ss.T(), got, want)

}

func (ss *ServerSuite) printLogs() {
	for _, e := range ss.hook.Entries {
		fmt.Printf("ENT[%s]: %s\n", e.Level, e.Message)
	}
}

func content(t *testing.T, mig *Migrator, file string) []byte {
	f := filepath.Join(mig.Root, file)
	fh, err := mig.FS.Open(f)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()

	s, err := ioutil.ReadAll(fh)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

/*
   t.Parallel()

        config, err := pgconn.ParseConfig(os.Getenv("PGX_TEST_CONN_STRING"))
        require.NoError(t, err)

*/
