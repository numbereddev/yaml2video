package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ondics/yaml2video/src/render"
)

type cliOptions struct {
	videoPath    string
	templatePath string
	render       render.Options
	dryRun       bool
	serveAPI     bool
}

func newFlagSet(options *cliOptions) *flag.FlagSet {
	flags := flag.NewFlagSet("yaml2video", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	flags.BoolVar(&options.dryRun, "n", false, "validate and preview the render plan and FFmpeg commands without rendering")
	flags.StringVar(&options.render.WorkDir, "work-dir", "", "directory for intermediate render files (default: .yaml2video-v2 beside the video YAML)")
	flags.StringVar(&options.render.Output, "o", "", "output file (default: output.mp4 beside the video YAML)")
	flags.StringVar(&options.templatePath, "t", "", "template YAML file (required for rendering)")
	return flags
}

func parseArgs() (cliOptions, error) {
	args := os.Args[1:]
	var options cliOptions
	if len(args) == 0 {
		return options, errors.New("a video YAML file or the serve command is required")
	}

	if args[0] == "serve" {
		if len(args) == 2 && (args[1] == "-h" || args[1] == "--help") {
			return options, flag.ErrHelp
		}
		if len(args) != 1 {
			return options, errors.New("serve does not accept additional arguments")
		}
		options.serveAPI = true
		return options, nil
	}

	// Go's flag parser stops at the first positional argument. Accept the
	// video path before the flags as well as after them.
	flagArgs := args
	if !strings.HasPrefix(args[0], "-") {
		options.videoPath = args[0]
		flagArgs = args[1:]
	}
	flags := newFlagSet(&options)
	if err := flags.Parse(flagArgs); err != nil {
		return options, err
	}

	if options.videoPath == "" {
		if flags.NArg() != 1 {
			return options, errors.New("exactly one video YAML file is required")
		}
		options.videoPath = flags.Arg(0)
	} else if flags.NArg() != 0 {
		return options, errors.New("exactly one video YAML file is required; put flags before or after the video path")
	}
	if options.templatePath == "" {
		return options, errors.New("a template YAML file is required (-t template.yaml)")
	}
	return options, nil
}

func shellArgs(args []string) string {
	var result strings.Builder
	for i, arg := range args {
		if i != 0 {
			result.WriteString(" ")
		}
		fmt.Fprintf(&result, "%q", arg)
	}
	return result.String()
}
