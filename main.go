package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ondics/yaml2video/project"
	"github.com/ondics/yaml2video/render"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

func main() {
	projectPath, options := parseArgs()
	value, err := project.Load(projectPath, project.LoadOptions{
		TemplatePaths: options.templatePaths,
	})
	if err != nil {
		fail(err)
	}

	plan, err := render.Compile(value, options.render)
	if err != nil {
		fail(err)
	}
	if options.dryRun {
		printSummary(projectPath, plan)
		ffmpeg.LogCompiledCommand = false
		commands, err := plan.Commands()
		if err != nil {
			fail(err)
		}
		fmt.Println("\nFFmpeg commands:")
		for _, args := range commands {
			fmt.Println(shellArgs(args))
		}
		return
	}

	if err := plan.Render(context.Background()); err != nil {
		fail(err)
	}
}

type cliOptions struct {
	render        render.Options
	templatePaths stringFlags
	dryRun        bool
}

type stringFlags []string

func (values *stringFlags) String() string {
	return strings.Join(*values, ", ")
}

func (values *stringFlags) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func parseArgs() (string, cliOptions) {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	var projectPath string
	flagArgs := args
	if !strings.HasPrefix(args[0], "-") {
		projectPath = args[0]
		flagArgs = args[1:]
	}

	flags := flag.NewFlagSet("yaml2video", flag.ExitOnError)
	dryRun := flags.Bool("n", false, "validate and print render details/FFmpeg commands without rendering")
	workDir := flags.String("work-dir", ".out", "directory for intermediate render files")
	output := flags.String("o", "", "final output file")
	var templatePaths stringFlags
	flags.Var(&templatePaths, "t", "slide template file; may be provided more than once")
	_ = flags.Parse(flagArgs)

	if projectPath == "" {
		if flags.NArg() != 1 {
			usage()
			os.Exit(2)
		}
		projectPath = flags.Arg(0)
	} else if flags.NArg() != 0 {
		usage()
		os.Exit(2)
	}
	return projectPath, cliOptions{
		render:        render.Options{WorkDir: *workDir, Output: *output},
		templatePaths: templatePaths,
		dryRun:        *dryRun,
	}
}

func printSummary(projectPath string, plan *render.Plan) {
	frames := int64(plan.Duration.Seconds()*float64(plan.Video.FPS) + 0.5)
	fmt.Printf("Project:    %s\n", projectPath)
	fmt.Printf("Output:     %s\n", plan.Output)
	fmt.Printf("Video:      %dx%d at %d fps\n", plan.Video.Width, plan.Video.Height, plan.Video.FPS)
	fmt.Printf("Timeline:   %s (%d frames)\n", plan.Duration, frames)
	fmt.Printf("Scenes:     %d\n", len(plan.Scenes))
	for i, scene := range plan.Scenes {
		fmt.Printf("  %02d  %-16s %s\n", i+1, scene.ID, scene.Duration)
	}
	if plan.Music == nil {
		fmt.Println("Music:      none")
	} else {
		fmt.Printf("Music:      %s (volume %.2f, fade out %s)\n", plan.Music.Path, plan.Music.Volume, plan.Music.FadeOut)
	}
	fmt.Printf("Work dir:   %s\n", plan.WorkDir)
	fmt.Printf("Transitions: %d\n", len(plan.Transitions))
}

func shellArgs(args []string) string {
	result := ""
	for i, arg := range args {
		if i != 0 {
			result += " "
		}
		result += fmt.Sprintf("%q", arg)
	}
	return result
}

func fail(err error) { fmt.Fprintln(os.Stderr, "yaml2video:", err); os.Exit(1) }
func usage() {
	fmt.Fprintln(os.Stderr, "usage: yaml2video [-n] [-t template.yaml]... [-work-dir .out] [-o output.mp4] <project.yaml>")
	fmt.Fprintln(os.Stderr, "       yaml2video <project.yaml> [-n] [-t template.yaml]... [-work-dir .out] [-o output.mp4]")
}
