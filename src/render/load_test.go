package render

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadBytes(t *testing.T) {
	videoPath, templatePath := example(t, "simple")
	want, err := Load(videoPath, templatePath)
	if err != nil {
		t.Fatal(err)
	}
	videoData, err := os.ReadFile(videoPath)
	if err != nil {
		t.Fatal(err)
	}
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{videoPath, templatePath} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}

	got, err := LoadBytes(videoData, templateData, filepath.Dir(videoPath))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("byte-based plan differs from file-based plan")
	}

	for _, tc := range []struct {
		name     string
		video    []byte
		template []byte
		message  string
	}{
		{"invalid video YAML", []byte("["), templateData, "video: YAML:"},
		{"invalid template YAML", videoData, []byte("["), "template: YAML:"},
		{"invalid video schema", []byte("version: 2\nunexpected: true\n"), templateData, "video:"},
		{"invalid template schema", videoData, []byte("version: 2\nunexpected: true\n"), "template:"},
		{"extra document", append(append([]byte(nil), videoData...), []byte("\n---\nversion: 2\n")...), templateData, "one YAML document"},
		{"template mismatch", []byte(strings.Replace(string(videoData), "office-mobility-simple", "other", 1)), templateData, "does not match"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadBytes(tc.video, tc.template, filepath.Dir(videoPath))
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("expected %q error, got %v", tc.message, err)
			}
		})
	}
}
