package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ondics/yaml2video/src/render"
	"github.com/ondics/yaml2video/src/server"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

func main() {
	options, err := parseArgs()
	if errors.Is(err, flag.ErrHelp) {
		printHelp()
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "yaml2video:", err)
		printHelp()
		os.Exit(2)
	}

	if options.serve {
		if err := server.Serve(); err != nil {
			fail(err)
		}
		return
	}

	ffmpeg.LogCompiledCommand = false
	plan, err := render.LoadWithTemplateBase(options.videoPath, options.templatePath, options.render.WorkDir)
	if err != nil {
		fail(err)
	}
	applyRenderOptions(plan, options.render)

	if options.dryRun {
		if err := printDryRun(options.videoPath, plan); err != nil {
			fail(err)
		}
		return
	}
	if err := renderCLI(context.Background(), plan, os.Stderr, os.Stdout); err != nil {
		fail(err)
	}
}

func applyRenderOptions(plan *render.Plan, options render.Options) {
	if options.WorkDir != "" {
		plan.WorkDir = options.WorkDir
		for i := range plan.Scenes {
			plan.Scenes[i].ASSPath = filepath.Join(plan.WorkDir, "ass", fmt.Sprintf("scene-%04d.ass", i))
			plan.Scenes[i].Output = filepath.Join(plan.WorkDir, "scenes", fmt.Sprintf("scene-%04d.mkv", i))
		}
	}

	if options.Output != "" {
		plan.Output = options.Output
	}
}

func printHelp() {
	fmt.Printf(`yaml2video — create MP4 videos from YAML content and templates.

The video YAML supplies text, images, and audio. The template YAML defines
layout, styling, timing defaults, and output dimensions. Their template IDs
must match. Relative media paths resolve from the video YAML's directory.
The template comes from -t or the video's template_file field. Relative
template_file paths resolve beside the video YAML, or from --work-dir if set.

Usage:
  yaml2video [options] video.yaml
  yaml2video video.yaml [options]
  yaml2video serve

Options:
`)
	flags := newFlagSet(&cliOptions{})
	flags.VisitAll(func(f *flag.Flag) {
		if f.Name == "h" {
			return
		}
		name := "-" + f.Name
		if len(f.Name) > 1 {
			name = "--" + f.Name
		}
		if f.Name == "help" {
			name = "-h, --help"
		}
		valueName, usage := flag.UnquoteUsage(f)
		if valueName != "" {
			name += " " + valueName
		}
		fmt.Printf("  %s\n        %s\n", name, usage)
	})
}

func printDryRun(videoPath string, plan *render.Plan) error {
	logCompiledCommand := ffmpeg.LogCompiledCommand
	ffmpeg.LogCompiledCommand = false
	defer func() { ffmpeg.LogCompiledCommand = logCompiledCommand }()

	// commands, err := plan.Commands()
	// if err != nil {
	// 	return err
	// }
	printSummary(videoPath, plan)
	// fmt.Println("\nFFmpeg commands:")
	// for _, args := range commands {
	// 	fmt.Println(shellArgs(args))
	// }
	return nil
}

func printSummary(videoPath string, plan *render.Plan) {
	frames := int64(plan.Duration.Seconds()*float64(plan.Video.FPS) + 0.5)
	fmt.Printf("Video:      %s\n", videoPath)
	fmt.Printf("Output:     %s\n", plan.Output)
	fmt.Printf("Format:     %dx%d at %d fps\n", plan.Video.Width, plan.Video.Height, plan.Video.FPS)
	fmt.Printf("Timeline:   %s (%d frames)\n", plan.Duration, frames)
	fmt.Printf("Scenes:     %d\n", len(plan.Scenes))
	for i, scene := range plan.Scenes {
		fmt.Printf("  %02d  %-16s %s – %s (%s)\n", i+1, scene.ID, scene.Start, scene.Start+scene.Duration, scene.Duration)
		for _, layer := range scene.Layers {
			if layer.Kind == "text" {
				for _, span := range layer.Spans {
					fmt.Printf("       text: %s\n", span.Content)
				}
			} else if layer.Path != "" {
				fmt.Printf("       %s: %s (alt: %q)\n", layer.Kind, layer.Path, layer.AltText)
			}
			for _, effect := range layer.Effects {
				fmt.Printf("       effect: %s\n", effect.Type)
			}
		}
		for _, audio := range scene.Audio {
			fmt.Printf("       audio: %s at %s (volume %.2f)\n", audio.Path, scene.Start+audio.Offset, audio.Volume)
		}
	}
	if plan.Music == nil {
		fmt.Println("Music:      none")
	} else {
		fmt.Printf("Music:      %s (volume %.2f, fade out %s)\n", plan.Music.Path, plan.Music.Volume, plan.Music.FadeOut)
	}
	fmt.Printf("Work dir:   %s\n", plan.WorkDir)
	fmt.Printf("Transitions: %d\n", len(plan.Transitions))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "yaml2video:", err)
	os.Exit(1)
}
