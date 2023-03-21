package pgmig

import (
	//"os"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wojas/genericr"
	"github.com/go-logr/logr"
)

func TestConnect(t *testing.T) {

	dsn := "postgres://jack:secret@localhost:2/mydb"
	sink := genericr.New(func(e genericr.Entry) {
		t.Log(e.String())
	})
	log := logr.New(sink)
	mig := New(log, Config{}, nil, "")
	ctx := context.Background()
	_, err := mig.Connect(ctx, dsn)
	assert.NotNil(t, err)
}
