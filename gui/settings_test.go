package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultSettingsAreValid(t *testing.T) {
	settings, err := validateSettings(defaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	if settings.ProjectRoot != `C:\Apple-Music-Downloader` {
		t.Fatalf("unexpected root: %s", settings.ProjectRoot)
	}
}

func TestValidateSettingsRejectsUnsafeValues(t *testing.T) {
	tests := []func(*Settings){
		func(value *Settings) { value.ProjectRoot = "relative" },
		func(value *Settings) { value.Storefront = "usa" },
		func(value *Settings) { value.AlacSaveFolder = "relative" },
		func(value *Settings) { value.CoverSize = "50000x1" },
		func(value *Settings) { value.AacType = "unknown" },
	}
	for _, mutate := range tests {
		value := defaultSettings()
		mutate(&value)
		if _, err := validateSettings(value); err == nil {
			t.Fatalf("expected invalid settings: %+v", value)
		}
	}
}

func TestSafeOverridesContainNoSensitiveKeys(t *testing.T) {
	path, err := writeSafeOverrides(t.TempDir(), defaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"mediaUserToken", "authorizationToken", "password", "proxy", "getAccountPort"} {
		if _, exists := value[key]; exists {
			t.Fatalf("sensitive key was persisted: %s", key)
		}
	}
	if filepath.Base(path) != "safe-overrides.json" {
		t.Fatalf("unexpected filename: %s", path)
	}
}

func TestDecodeUTF16WSLOutput(t *testing.T) {
	input := []byte{'U', 0, 'b', 0, 'u', 0, 'n', 0, 't', 0, 'u', 0, '\r', 0, '\n', 0}
	if got := decodeWindowsCommandOutput(input); got != "Ubuntu\r\n" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestWindowsPathToWSL(t *testing.T) {
	if got := windowsPathToWSL(`C:\Apple-Music-Downloader\wrapper-runtime`); got != "/mnt/c/Apple-Music-Downloader/wrapper-runtime" {
		t.Fatalf("unexpected WSL path: %s", got)
	}
}

func TestPersistedSettingsDoNotContainCredentialWords(t *testing.T) {
	dir := t.TempDir()
	if err := persistSettings(dir, defaultSettings()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(data))
	for _, forbidden := range []string{"password", "token", "appleid", "2fa"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("settings contain credential-related key %q", forbidden)
		}
	}
}
