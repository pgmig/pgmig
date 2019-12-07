package main

import (
	"log"
)

func run(exitFunc func(code int)) {
	var err error
	var cfg *Config
	defer func() { shutdown(exitFunc, err) }()
	cfg, err = setupConfig()
	if err != nil {
		return
	}
	l := setupLog(cfg)
	mig := setupMig(cfg, l)
	committed, err := mig.Run(cfg.Args.Command, cfg.Args.Packages)
	if err == nil { // shutdown shows error otherwise
		log.Printf("Saved: %v", *committed)
	}
}

// exit after deferred cleanups have run
func shutdown(exitFunc func(code int), e error) {
	if e != nil {
		var code int
		switch e {
		case ErrGotHelp:
			code = 3
		case ErrBadArgs:
			code = 2
		default:
			code = 1
			log.Printf("Run error: %+v", e.Error())
		}
		exitFunc(code)
	}
}
