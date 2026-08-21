package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
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
