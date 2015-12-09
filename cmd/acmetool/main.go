package main

import (
	"fmt"
	"github.com/hlandau/acme/interaction"
	"github.com/hlandau/acme/notify"
	"github.com/hlandau/acme/storage"
	"github.com/hlandau/degoutils/xlogconfig"
	"github.com/hlandau/xlog"
	"github.com/square/go-jose"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/hlandau/easyconfig.v1/adaptflag"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
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

	reconcileCmd = kingpin.Command("reconcile", "Reconcile ACME state").Default()

	wantCmd = kingpin.Command("want", "Add a target with one or more hostnames")
	wantArg = wantCmd.Arg("hostname", "hostnames for which a certificate is desired").Required().Strings()

	quickstartCmd = kingpin.Command("quickstart", "Interactively ask some getting started questions (recommended)")
	expertFlag    = quickstartCmd.Flag("expert", "Ask more questions in quickstart wizard").Bool()

	importJWKAccountCmd = kingpin.Command("import-jwk-account", "Import a JWK account key")
	importJWKURLArg     = importJWKAccountCmd.Arg("provider-url", "Provider URL (e.g. https://acme-v01.api.letsencrypt.org/directory)").Required().String()
	importJWKPathArg    = importJWKAccountCmd.Arg("private-key-file", "Path to private_key.json").Required().ExistingFile()

	importKeyCmd = kingpin.Command("import-key", "Import a certificate private key")
	importKeyArg = importKeyCmd.Arg("private-key-file", "Path to PEM-encoded private key").Required().ExistingFile()

	importLECmd = kingpin.Command("import-le", "Import a Let's Encrypt client state directory")
	importLEArg = importLECmd.Arg("le-state-path", "Path to Let's Encrypt state directory").Default("/etc/letsencrypt").ExistingDir()
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
	case "reconcile":
		cmdReconcile()
	case "want":
		cmdWant()
		cmdReconcile()
	case "quickstart":
		cmdQuickstart()
	case "import-key":
		cmdImportKey()
	case "import-jwk-account":
		cmdImportJWKAccount()
	case "import-le":
		cmdImportLE()
		cmdReconcile()
	}
}

func cmdImportJWKAccount() {
	s, err := storage.New(*stateFlag)
	log.Fatale(err, "storage")

	f, err := os.Open(*importJWKPathArg)
	log.Fatale(err, "cannot open private key file")
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	log.Fatale(err, "cannot read file")

	k := jose.JsonWebKey{}
	err = k.UnmarshalJSON(b)
	log.Fatale(err, "cannot unmarshal key")

	err = s.ImportAccountKey(*importJWKURLArg, k.Key)
	log.Fatale(err, "cannot import account key")
}

func cmdImportKey() {
	s, err := storage.New(*stateFlag)
	log.Fatale(err, "storage")

	err = importKey(s, *importKeyArg)
	log.Fatale(err, "import key")
}

func cmdReconcile() {
	s, err := storage.New(*stateFlag)
	log.Fatale(err, "storage")

	err = s.Reconcile()
	log.Fatale(err, "reconcile")
}

func cmdWant() {
	s, err := storage.New(*stateFlag)
	log.Fatale(err, "storage")

	tgt := storage.Target{
		Names: *wantArg,
	}

	err = s.AddTarget(tgt)
	log.Fatale(err, "add target")
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
