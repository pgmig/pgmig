package main

import (
	"errors"
	//	"fmt"

	"github.com/jessevdk/go-flags"

	mapper "github.com/birkirb/loggers-mapper-logrus"
	"github.com/sirupsen/logrus"
	"gopkg.in/birkirb/loggers.v1"

	"github.com/pgmig/pgmig"
)

// Config holds all config vars
type Config struct {
	Verbose bool `long:"verbose" description:"Show debug data"`
	Args    struct {
		Command  string   `choice:"create" choice:"test" choice:"clean" choice:"drop" choice:"recreate" description:"create|test|clean|drop|recreate"`
		Packages []string `description:"dirnames under SQL sources directory in create order"`
	} `positional-args:"yes" required:"yes"`
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
		//fmt.Printf("Args error: %#v", err)
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

	mig := pgmig.New(cfg.Mig, log, nil)
	return mig
}
