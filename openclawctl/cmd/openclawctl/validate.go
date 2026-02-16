package main

import (
	"flag"
)

func runValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	manifest, err := loadManifest()
	if err != nil {
		return err
	}
	outSuccessf("Manifest valid: %s (%s=%s)", accent(manifest.Metadata.Name), keyLabel("profile"), accent(globalOpts.Profile))
	return nil
}
