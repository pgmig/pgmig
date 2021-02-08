package main_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	cmd "github.com/pgmig/pgmig/cmd/pgmig"
)

const (
	PgDsnEnv = "TEST_DSN_PG"
)

func TestRunErrors(t *testing.T) {
	// Save original args
	a := os.Args

	tests := []struct {
		name string
		code int
		args []string
	}{
		{"Help", 3, []string{"-h"}},
		{"UnknownFlag", 2, []string{"-0"}},
	}
	for _, tt := range tests {
		os.Args = append([]string{a[0]}, tt.args...)
		var c int
		cmd.Run(func(code int) { c = code })
		assert.Equal(t, tt.code, c, tt.name)
	}
	// Restore original args
	os.Args = a
}

func TestRun(t *testing.T) {
	pgDSN := os.Getenv(PgDsnEnv)
	if pgDSN == "" {
		t.Skip("Skipping testing when DSN is empty")
	}
	// Save original args
	a := os.Args
	os.Args = append([]string{a[0]}, "--debug",
		"--dsn", pgDSN,
		"init",
	)
	var c int
	cmd.Run(func(code int) { c = code })
	assert.Equal(t, 0, c, "Normal run")
	// Restore original args
	os.Args = a
}
