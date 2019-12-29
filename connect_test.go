package pgmig

import (
	//"os"
	"github.com/stretchr/testify/assert"
	"testing"

	mapper "github.com/birkirb/loggers-mapper-logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

func TestConnect(t *testing.T) {

	dsn := "postgres://jack:secret@localhost:2/mydb"

	l, _ := test.NewNullLogger()
	log := mapper.NewLogger(l)
	mig := New(Config{}, log, nil, "")

	_, err := mig.Connect("unknown")
	assert.NotNil(t, err)
	_, err = mig.Connect(dsn)
	assert.NotNil(t, err)
}
