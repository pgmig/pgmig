//go:generate mockgen -destination=generated_mock_test.go -package pgmig github.com/jackc/pgx/v4 Tx,Rows

package pgmig

import (
	"context"
	"fmt"
	"github.com/golang/mock/gomock"
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

	ss.srv = New(ss.cfg, log, nil)

	GitVersion(".", &version)
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
	fmt.Println(ct0)

	gomock.InOrder(
		tx.EXPECT().Query(ctx, SQLPgMigExists, "pgmig", "pkg").Return(rv, nil),
		tx.EXPECT().Exec(ctx, "SELECT pgmig.pkg_op_before(a_op => $1, a_code => $2, a_version => $3, a_repo => $4)", "init", "a", version, "git@github.com:pgmig/pgmig.git").Return(ct0, nil),
		tx.EXPECT().Exec(ctx, "SELECT 'init'"), //.Return(ct0, nil),
		tx.EXPECT().Exec(ctx, "SELECT 'ddl1';\nSELECT 'ddl2';\n"),
		tx.EXPECT().Exec(ctx, "SELECT 'ddl test';\n"),
		tx.EXPECT().Query(ctx, "SELECT pgmig.script_protected(a_pkg => $1, a_file => $2)", "a", "03.once.sql").Return(rv, nil),
		tx.EXPECT().Exec(ctx, "SELECT pgmig.script_protect(a_pkg => $1, a_file => $2, a_md5 => $3)", "a", "03.once.sql", "8f422d0ba69c33f13825fb28d682f288"),
		tx.EXPECT().Exec(ctx, "SELECT 'once';\n"),
		tx.EXPECT().Exec(ctx, "SELECT 'new';\n"),
		tx.EXPECT().Exec(ctx, "SELECT pgmig.pkg_op_after(a_op => $1, a_code => $2, a_version => $3, a_repo => $4)", "init", "a", version, "git@github.com:pgmig/pgmig.git"),
	)

	ss.srv.Config.Dir = "testdata"

	commit, err := ss.srv.Run(tx, "init", []string{"a"}) // []string{"a", "b"})

	//	ss.printLogs()
	assert.Nil(ss.T(), err)
	assert.Equal(ss.T(), *commit, true)

}

func (ss *ServerSuite) printLogs() {
	for _, e := range ss.hook.Entries {
		fmt.Printf("ENT[%s]: %s\n", e.Level, e.Message)
	}
}
