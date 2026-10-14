package main

import (
	"os"
	"testing"
)

func TestParseArgsOptionalTemplate(t *testing.T) {
	original := os.Args
	t.Cleanup(func() { os.Args = original })
	for _, tc := range []struct {
		name     string
		args     []string
		template string
		workDir  string
	}{
		{"no template flag", []string{"video.yaml"}, "", ""},
		{"explicit template", []string{"-t", "template.yaml", "video.yaml"}, "template.yaml", ""},
		{"work dir before video", []string{"--work-dir", "work", "video.yaml"}, "", "work"},
		{"work dir after video", []string{"video.yaml", "--work-dir", "work"}, "", "work"},
		{"dry run without template flag", []string{"-n", "video.yaml"}, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			os.Args = append([]string{"yaml2video"}, tc.args...)
			got, err := parseArgs()
			if err != nil {
				t.Fatal(err)
			}
			if got.videoPath != "video.yaml" || got.templatePath != tc.template || got.render.WorkDir != tc.workDir {
				t.Fatalf("unexpected options: %+v", got)
			}
		})
	}
}
