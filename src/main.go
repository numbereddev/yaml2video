package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ondics/yaml2video/src/api"
	"github.com/ondics/yaml2video/src/render"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

const serverPort = 8080

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

	if options.serveAPI {
		if err := serve(); err != nil {
			fail(err)
		}
		return
	}

	plan, err := render.Load(options.videoPath, options.templatePath)
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
	if err := plan.Render(context.Background()); err != nil {
		fail(err)
	}
}

func serve() error {
	store, err := api.NewServer(api.ConfigFromEnv())
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go store.RunCleanup(ctx)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", serverPort),
		Handler:           store.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
		// WriteTimeout is deliberately unset for long-running SSE renders.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	shutdown := make(chan struct{})

	go func() {
		<-ctx.Done()
		timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(timeout); err != nil {
			log.Printf("HTTP shutdown: %v", err)
			_ = server.Close()
		}
		close(shutdown)
	}()

	log.Printf("HTTP server listening on %s", server.Addr)
	err = server.ListenAndServe()
	stop()
	<-shutdown
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
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

Usage:
  yaml2video [options] -t template.yaml video.yaml
  yaml2video video.yaml [options] -t template.yaml
  yaml2video serve

Options:
  -h, --help
        show this help page
`)
	flags := newFlagSet(&cliOptions{})
	flags.SetOutput(os.Stdout)
	flags.PrintDefaults()
}

func printDryRun(videoPath string, plan *render.Plan) error {
	logCompiledCommand := ffmpeg.LogCompiledCommand
	ffmpeg.LogCompiledCommand = false
	defer func() { ffmpeg.LogCompiledCommand = logCompiledCommand }()

	commands, err := plan.Commands()
	if err != nil {
		return err
	}
	printSummary(videoPath, plan)
	fmt.Println("\nFFmpeg commands:")
	for _, args := range commands {
		fmt.Println(shellArgs(args))
	}
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
