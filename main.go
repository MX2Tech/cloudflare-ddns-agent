package main

import (
	"fmt"
	"os"
	"time"

	"github.com/MX2Tech/cloudflare-ddns-agent/internal/cloudflare"
	"github.com/MX2Tech/cloudflare-ddns-agent/internal/config"
	"github.com/MX2Tech/cloudflare-ddns-agent/internal/loop"
	"github.com/MX2Tech/cloudflare-ddns-agent/internal/publicip"
	"github.com/MX2Tech/cloudflare-ddns-agent/internal/updater"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "update":
		os.Exit(runUpdate(defaultConfigPath()))
	case "run":
		os.Exit(runLoop(defaultConfigPath()))
	case "install":
		os.Exit(runInstall(defaultConfigPath()))
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
	fmt.Fprintf(os.Stderr, `cloudflare-ddns-agent - keeps DNS A records pointed at this machine's public IP

Usage:
  cloudflare-ddns-agent update     Check the public IP and reconcile all configured records once
  cloudflare-ddns-agent run        Reconcile forever, every check_interval (used by the Windows task)
  cloudflare-ddns-agent install    %s
  cloudflare-ddns-agent uninstall  %s

Config: %s
`, installDescription, uninstallDescription, defaultConfigPath())
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
			fmt.Fprintln(os.Stderr, loop.FormatResult(r))
			exitCode = 1
			continue
		}
		fmt.Println(loop.FormatResult(r))
	}
	return exitCode
}

func runLoop(configPath string) int {
	out, closeLog, err := openRunLog()
	if err != nil {
		fmt.Fprintf(os.Stderr, "level=error msg=%q\n", err.Error())
		return 1
	}
	defer closeLog()

	loop.Run(loop.Options{
		LoadConfig: func() (*config.Config, error) { return config.Load(configPath) },
		NewClient: func(apiToken string) updater.CloudflareClient {
			return cloudflare.NewClient(apiToken)
		},
		DetectIP: publicip.Detect,
		Log: func(line string) {
			fmt.Fprintf(out, "%s %s\n", time.Now().Format(time.RFC3339), line)
		},
		Sleep: time.Sleep,
	})
	return 0
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

	if err := installService(interval); err != nil {
		fmt.Fprintf(os.Stderr, "level=error msg=%q\n", err.Error())
		return 1
	}

	fmt.Printf("level=info msg=%q\n", installedMessage)
	return 0
}

func runUninstall() int {
	if err := uninstallService(); err != nil {
		fmt.Fprintf(os.Stderr, "level=error msg=%q\n", err.Error())
		return 1
	}
	fmt.Printf("level=info msg=%q\n", uninstalledMessage)
	return 0
}
