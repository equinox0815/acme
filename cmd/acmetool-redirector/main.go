package main

import (
	"fmt"
	"github.com/hlandau/acme/interaction"
	"github.com/hlandau/acme/notify"
	"github.com/hlandau/acme/redirector"
	"github.com/hlandau/acme/storage"
	"github.com/hlandau/degoutils/xlogconfig"
	"github.com/hlandau/xlog"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/hlandau/easyconfig.v1/adaptflag"
	"gopkg.in/hlandau/service.v2"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
	"strings"
)

var log, Log = xlog.New("acmetool")

var (
	stateFlag = kingpin.Flag("state", "Path to the state directory (env: ACME_STATE_DIR)").
			Default(storage.RecommendedPath).
			Envar("ACME_STATE_DIR").
			PlaceHolder(storage.RecommendedPath).
			String()

	hooksFlag = kingpin.Flag("hooks", "Path to the notification hooks directory (env: ACME_HOOKS_DIR)").
			Default(notify.DefaultHookPath).
			Envar("ACME_HOOKS_DIR").
			PlaceHolder(notify.DefaultHookPath).
			String()

	batchFlag = kingpin.Flag("batch", "Do not attempt interaction; useful for cron jobs").
			Bool()

	stdioFlag = kingpin.Flag("stdio", "Don't attempt to use console dialogs; fall back to stdio prompts").Bool()

	responseFileFlag = kingpin.Flag("response-file", "Read dialog responses from the given file").ExistingFile()

	redirectorCmd      = kingpin.Command("run", "HTTP to HTTPS redirector with challenge response support")
	redirectorPathFlag = redirectorCmd.Flag("path", "Path to serve challenge files from").String()
	redirectorGIDFlag  = redirectorCmd.Flag("challenge-gid", "GID to chgrp the challenge path to (optional)").String()
)

func main() {
	adaptflag.Adapt()
	cmd := kingpin.Parse()
	xlogconfig.Init()

	if *batchFlag {
		interaction.NonInteractive = true
	}

	if *stdioFlag {
		interaction.NoDialog = true
	}

	if *responseFileFlag != "" {
		err := loadResponseFile(*responseFileFlag)
		log.Errore(err, "cannot load response file, continuing anyway")
	}

	switch cmd {
	case "run":
		cmdRunRedirector()
	}
}

func cmdRunRedirector() {
	rpath := *redirectorPathFlag
	if rpath == "" {
		rpath = determineWebroot()
	}

	service.Main(&service.Info{
		Name:          "acmetool",
		Description:   "acmetool HTTP redirector",
		DefaultChroot: rpath,
		NewFunc: func() (service.Runnable, error) {
			return redirector.New(redirector.Config{
				Bind:          ":80",
				ChallengePath: rpath,
				ChallengeGID:  *redirectorGIDFlag,
			})
		},
	})
}

func determineWebroot() string {
	// don't use fdb for this, we don't need access to the whole db
	b, err := ioutil.ReadFile(filepath.Join(*stateFlag, "conf", "webroot-path"))
	if err == nil {
		s := strings.TrimSpace(strings.Split(strings.TrimSpace(string(b)), "\n")[0])
		if s != "" {
			return s
		}
	}

	return "/var/run/acme/acme-challenge"
}

// YAML response file loading.

func loadResponseFile(path string) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	m := map[string]interface{}{}
	err = yaml.Unmarshal(b, &m)
	if err != nil {
		return err
	}

	for k, v := range m {
		r, err := parseResponse(v)
		if err != nil {
			log.Errore(err, "response for ", k, " invalid")
			continue
		}
		interaction.SetResponse(k, r)
	}

	return nil
}

func parseResponse(v interface{}) (*interaction.Response, error) {
	switch x := v.(type) {
	case string:
		return &interaction.Response{
			Value: x,
		}, nil
	case int:
		return &interaction.Response{
			Value: fmt.Sprintf("%d", x),
		}, nil
	case bool:
		return &interaction.Response{
			Cancelled: !x,
		}, nil
	default:
		return nil, fmt.Errorf("unknown response value")
	}
}

// © 2015 Hugo Landau <hlandau@devever.net>    MIT License
