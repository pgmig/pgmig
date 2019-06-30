package main

import (
	"errors"

	"github.com/jessevdk/go-flags"

	mapper "github.com/birkirb/loggers-mapper-logrus"
	"github.com/sirupsen/logrus"
	"gopkg.in/birkirb/loggers.v1"

	"github.com/pgmig/pgmig"
)

// Config holds all config vars
type Config struct {
	Addr        string `long:"http_addr" default:"localhost:8080"  description:"Http listen address"`
	UploadLimit int64  `long:"upload_limit" default:"8" description:"Upload size limit (Mb)"`
	Verbose     bool   `long:"verbose" description:"Show debug data"`

	Mig pgmig.Config `group:"Migrator Options" namespace:"mig"`
}

var (
	// ErrGotHelp returned after showing requested help
	ErrGotHelp = errors.New("help printed")
	// ErrBadArgs returned after showing command args error message
	ErrBadArgs = errors.New("option error printed")
)

// setupConfig loads flags from args (if given) or command flags and ENV otherwise
func setupConfig(args ...string) (*Config, error) {
	cfg := &Config{}
	p := flags.NewParser(cfg, flags.Default)
	var err error
	if len(args) == 0 {
		_, err = p.Parse()
	} else {
		_, err = p.ParseArgs(args)
	}
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			return nil, ErrGotHelp
		}
		return nil, ErrBadArgs
	}
	return cfg, nil
}

// setupLog creates logger
func setupLog(cfg *Config) loggers.Contextual {
	l := logrus.New()
	if cfg.Verbose {
		l.SetLevel(logrus.DebugLevel)
		l.SetReportCaller(true)
	}
	return &mapper.Logger{Logger: l} // Same as mapper.NewLogger(l) but without info log message
}

// setupMig creates pg migrator instance
func setupMig(cfg *Config, log loggers.Contextual) *pgmig.Migrator {
	mig := pgmig.New(cfg.Mig, log)
	return mig
}
