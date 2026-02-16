package main

import (
	"context"
	"flag"

	"github.com/kangeunchan/openclawctl/internal/runtime"
)

func runLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	var follow bool
	fs.BoolVar(&follow, "follow", false, "follow log output")
	fs.BoolVar(&follow, "f", false, "follow log output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	manifest, err := loadManifest()
	if err != nil {
		return err
	}
	containerName := currentContainerName(manifest)
	return runtime.NewDockerRuntime().Logs(context.Background(), containerName, follow)
}
