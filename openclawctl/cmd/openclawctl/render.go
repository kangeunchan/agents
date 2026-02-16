package main

import (
	"flag"
	"fmt"
)

func runRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	var outputPath string
	fs.StringVar(&outputPath, "output", "", "output path for rendered openclaw.json (stdout if empty)")
	fs.StringVar(&outputPath, "o", "", "output path for rendered openclaw.json (stdout if empty)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := expandFlagEnv("--output", &outputPath); err != nil {
		return err
	}

	manifest, err := loadManifest()
	if err != nil {
		return err
	}
	bytesData, _, err := renderManifest(manifest)
	if err != nil {
		return err
	}

	if outputPath == "" {
		fmt.Print(string(bytesData))
		return nil
	}
	if err := writeFile(outputPath, bytesData); err != nil {
		return err
	}
	outSuccessf("Rendered config written to %s", accent(outputPath))
	return nil
}
