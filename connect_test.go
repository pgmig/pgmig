package pgmig

import (
	//"os"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wojas/genericr"
)

func TestConnect(t *testing.T) {

	dsn := "postgres://jack:secret@localhost:2/mydb"
	log := genericr.New(func(e genericr.Entry) {
		//t.Log(e.String())
	})
	mig := New(log, Config{}, nil, "")
	ctx := context.Background()
	_, err := mig.Connect(ctx, dsn)
	assert.NotNil(t, err)
}
