// +build linux

package main

import (
	sddbus "github.com/coreos/go-systemd/dbus"
	sdunit "github.com/coreos/go-systemd/unit"
	sdutil "github.com/coreos/go-systemd/util"
	"gopkg.in/hlandau/svcutils.v1/exepath"
	"io"
	"os"
)

func cmdInstallRedirector() {
	if !sdutil.IsRunningSystemd() {
		log.Debugf("not running systemd")
		return
	}

	log.Debug("connecting to systemd")
	conn, err := sddbus.New()
	if err != nil {
		log.Errore(err, "connect to systemd")
		return
	}

	defer conn.Close()
	log.Debug("connected")

	props, err := conn.GetUnitProperties("acmetool-redirector.service")
	if err != nil {
		log.Errore(err, "systemd GetUnitProperties")
		return
	}

	if props["LoadState"].(string) != "not-found" {
		log.Info("acmetool-redirector.service unit already installed, skipping")
		return
	}

	username, err := determineAppropriateUsername()
	if err != nil {
		log.Errore(err, "determine appropriate username")
		return
	}

	f, err := os.OpenFile("/etc/systemd/system/acmetool-redirector.service", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		log.Errore(err, "acmetool-redirector.service unit file already exists?")
		return
	}
	defer f.Close()

	rdr := sdunit.Serialize([]*sdunit.UnitOption{
		sdunit.NewUnitOption("Unit", "Description", "acmetool HTTP redirector"),
		sdunit.NewUnitOption("Service", "Type", "notify"),
		sdunit.NewUnitOption("Service", "ExecStart", exepath.Abs+` run --service.uid=`+username),
		sdunit.NewUnitOption("Service", "Restart", "always"),
		sdunit.NewUnitOption("Service", "RestartSec", "30"),
		sdunit.NewUnitOption("Install", "WantedBy", "multi-user.target"),
	})

	_, err = io.Copy(f, rdr)
	if err != nil {
		log.Errore(err, "cannot write unit file")
		return
	}

	f.Close()
	err = conn.Reload() // softfail
	log.Warne(err, "systemctl daemon-reload failed")

	_, _, err = conn.EnableUnitFiles([]string{"acmetool-redirector.service"}, false, false)
	log.Errore(err, "failed to enable unit acmetool-redirector.service")

	_, err = conn.StartUnit("acmetool-redirector.service", "replace", nil)
	log.Errore(err, "failed to start acmetool-redirector")
	resultStr := "The acmetool-redirector service was successfully started."
	if err != nil {
		resultStr = "The acmetool-redirector service WAS NOT successfully started. You may have a web server listening on port 80. You will need to troubleshoot this yourself."
	}
	log.Debug(resultStr)
}
