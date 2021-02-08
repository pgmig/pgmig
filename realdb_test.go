package pgmig_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wojas/genericr"
)

const (
	PgDsnEnv = "TEST_DSN_PG"
)

func TestRunPlugins(t *testing.T) {
	log := genericr.New(func(e genericr.Entry) {
		t.Log(e.String())
	})
	pgDSN := os.Getenv(PgDsnEnv)
	if pgDSN == "" {
		t.Skip("Skipping testing when DSN is empty")
	}
	log.Info("Setting up database")
	require.NotEmpty(t, t)
}
