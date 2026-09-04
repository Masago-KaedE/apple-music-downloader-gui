package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zhaarey/apple-music-downloader/utils/structs"
)

func TestPreparseFlagValue(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "default", args: []string{"--aac"}, want: "config.yaml"},
		{name: "separate", args: []string{"--config", `C:\test\config.yaml`}, want: `C:\test\config.yaml`},
		{name: "equals", args: []string{"--config=C:\\test\\config.yaml"}, want: `C:\test\config.yaml`},
		{name: "last wins", args: []string{"--config=one", "--config", "two"}, want: "two"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := preparseFlagValue(tt.args, "config", "config.yaml"); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHasFlagArg(t *testing.T) {
	if !hasFlagArg([]string{"--aac", "--help"}, "help", "h") {
		t.Fatal("expected --help to be detected")
	}
	if !hasFlagArg([]string{"-h"}, "help", "h") {
		t.Fatal("expected -h to be detected")
	}
	if hasFlagArg([]string{"--helpful"}, "help", "h") {
		t.Fatal("partial flag must not match")
	}
}

func TestApplySafeOverrides(t *testing.T) {
	original := Config
	t.Cleanup(func() { Config = original })
	Config = structs.ConfigSet{Storefront: "us", MediaUserToken: "must-remain"}

	dir := t.TempDir()
	path := filepath.Join(dir, "safe.json")
	data := []byte(`{"storefront":"jp","alacSaveFolder":"C:\\Music","embedCover":false,"aacType":"aac-binaural","alacMax":96000}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := applySafeOverrides(path); err != nil {
		t.Fatal(err)
	}
	if Config.Storefront != "jp" || Config.EmbedCover || Config.AacType != "aac-binaural" || Config.AlacMax != 96000 {
		t.Fatalf("safe overrides were not applied: %+v", Config)
	}
	if Config.MediaUserToken != "must-remain" {
		t.Fatal("sensitive configuration was changed")
	}
}

func TestSafeOverridesRejectUnknownOrUnsafeValues(t *testing.T) {
	tests := []string{
		`{"mediaUserToken":"secret"}`,
		`{"storefront":"usa"}`,
		`{"alacSaveFolder":"relative"}`,
		`{"aacType":"unknown"}`,
	}
	for _, body := range tests {
		dir := t.TempDir()
		path := filepath.Join(dir, "safe.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := applySafeOverrides(path); err == nil {
			t.Fatalf("expected %s to fail", body)
		}
	}
}
