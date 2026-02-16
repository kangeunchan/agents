package main

import (
	"fmt"
	"os"
)

func main() {
	command, commandArgs, showVersion, err := parseRootArgs(os.Args[1:])
	if err != nil {
		outErrorf("%v", err)
		os.Exit(2)
	}

	if showVersion && command == "" {
		fmt.Println(buildVersion())
		return
	}

	if command == "" {
		fmt.Fprint(os.Stderr, usage())
		os.Exit(2)
	}

	if err := dispatch(command, commandArgs); err != nil {
		outErrorf("%v", err)
		os.Exit(1)
	}
}

func dispatch(command string, args []string) error {
	switch command {
	case "validate":
		return runValidate(args)
	case "render":
		return runRender(args)
	case "diff":
		return runDiff(args)
	case "apply":
		return runApply(args)
	case "status":
		return runStatus(args)
	case "up":
		return runUp(args)
	case "down":
		return runDown(args)
	case "logs":
		return runLogs(args)
	case "devices":
		return runDevices(args)
	case "oauth":
		return runOAuth(args)
	case "version":
		fmt.Println(buildVersion())
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", command, usage())
	}
}
