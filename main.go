package main

import (
	"fmt"
	"os"

	"github.com/MX2Tech/cloudflare-ddns-agent/internal/cloudflare"
	"github.com/MX2Tech/cloudflare-ddns-agent/internal/config"
	"github.com/MX2Tech/cloudflare-ddns-agent/internal/publicip"
	"github.com/MX2Tech/cloudflare-ddns-agent/internal/systemdinstall"
	"github.com/MX2Tech/cloudflare-ddns-agent/internal/updater"
)

const defaultConfigPath = "/etc/cloudflare-ddns-agent/config.yaml"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "update":
		os.Exit(runUpdate(defaultConfigPath))
	case "install":
		os.Exit(runInstall(defaultConfigPath))
	case "uninstall":
		os.Exit(runUninstall())
	case "-h", "--help", "help":
		printUsage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `cloudflare-ddns-agent - keeps DNS A records pointed at this machine's public IP

Usage:
  cloudflare-ddns-agent update     Check the public IP and reconcile all configured records
  cloudflare-ddns-agent install    Install and enable the systemd timer
  cloudflare-ddns-agent uninstall  Stop and remove the systemd timer`)
}

func runUpdate(configPath string) int {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "level=error msg=%q\n", err.Error())
		return 1
	}

	client := cloudflare.NewClient(cfg.Cloudflare.APIToken)
	results := updater.Run(cfg, client, publicip.Detect)

	exitCode := 0
	for _, r := range results {
		if r.Action == "error" {
			fmt.Fprintf(os.Stderr, "level=error hostname=%s msg=%q\n", r.Hostname, r.Err.Error())
			exitCode = 1
			continue
		}
		fmt.Printf("level=info hostname=%s action=%s ip=%s\n", r.Hostname, r.Action, r.IP)
	}
	return exitCode
}

func runInstall(configPath string) int {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "level=error msg=%q\n", err.Error())
		return 1
	}

	interval, err := cfg.Interval()
	if err != nil {
		fmt.Fprintf(os.Stderr, "level=error msg=%q\n", err.Error())
		return 1
	}

	if err := systemdinstall.Install(interval); err != nil {
		fmt.Fprintf(os.Stderr, "level=error msg=%q\n", err.Error())
		return 1
	}

	fmt.Println(`level=info msg="cloudflare-ddns-agent installed and timer enabled"`)
	return 0
}

func runUninstall() int {
	if err := systemdinstall.Uninstall(); err != nil {
		fmt.Fprintf(os.Stderr, "level=error msg=%q\n", err.Error())
		return 1
	}
	fmt.Println(`level=info msg="cloudflare-ddns-agent timer removed"`)
	return 0
}
