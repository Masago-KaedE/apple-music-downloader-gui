package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	exitOK             = 0
	exitDownloadFailed = 1
	exitInvalidInput   = 2
	exitCanceled       = 130
	machineEventPrefix = "AMDL_EVENT "
)

var (
	machine_events  bool
	non_interactive bool
	machineJobID    string
	machineEventMu  sync.Mutex
)

type machineEvent struct {
	Version   int         `json:"version"`
	Type      string      `json:"type"`
	Timestamp string      `json:"timestamp"`
	JobID     string      `json:"jobId"`
	Payload   interface{} `json:"payload,omitempty"`
}

// SafeOverrides is deliberately a strict allow-list. Sensitive configuration
// such as account tokens, proxy values and Wrapper endpoints cannot be supplied
// through this file.
type SafeOverrides struct {
	Storefront      *string `json:"storefront,omitempty"`
	AlacSaveFolder  *string `json:"alacSaveFolder,omitempty"`
	AtmosSaveFolder *string `json:"atmosSaveFolder,omitempty"`
	AacSaveFolder   *string `json:"aacSaveFolder,omitempty"`
	EmbedCover      *bool   `json:"embedCover,omitempty"`
	CoverSize       *string `json:"coverSize,omitempty"`
	CoverFormat     *string `json:"coverFormat,omitempty"`
	EmbedLyrics     *bool   `json:"embedLyrics,omitempty"`
	SaveLyricsFile  *bool   `json:"saveLyricsFile,omitempty"`
	LyricsFormat    *string `json:"lyricsFormat,omitempty"`
	AacType         *string `json:"aacType,omitempty"`
	AlacMax         *int    `json:"alacMax,omitempty"`
	AtmosMax        *int    `json:"atmosMax,omitempty"`
}

func preparseFlagValue(args []string, name, fallback string) string {
	longName := "--" + name
	value := fallback
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, longName+"=") {
			value = strings.TrimPrefix(arg, longName+"=")
			continue
		}
		if arg == longName && i+1 < len(args) {
			value = args[i+1]
			i++
		}
	}
	return value
}

func hasFlagArg(args []string, names ...string) bool {
	for _, arg := range args {
		for _, name := range names {
			if arg == "--"+name || (len(name) == 1 && arg == "-"+name) {
				return true
			}
		}
	}
	return false
}

func applySafeOverrides(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read overrides: %w", err)
	}
	if len(data) > 1024*1024 {
		return fmt.Errorf("overrides file is too large")
	}

	var overrides SafeOverrides
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&overrides); err != nil {
		return fmt.Errorf("parse overrides: %w", err)
	}
	if err := validateSafeOverrides(&overrides); err != nil {
		return err
	}

	if overrides.Storefront != nil {
		Config.Storefront = strings.ToLower(*overrides.Storefront)
	}
	if overrides.AlacSaveFolder != nil {
		Config.AlacSaveFolder = *overrides.AlacSaveFolder
	}
	if overrides.AtmosSaveFolder != nil {
		Config.AtmosSaveFolder = *overrides.AtmosSaveFolder
	}
	if overrides.AacSaveFolder != nil {
		Config.AacSaveFolder = *overrides.AacSaveFolder
	}
	if overrides.EmbedCover != nil {
		Config.EmbedCover = *overrides.EmbedCover
	}
	if overrides.CoverSize != nil {
		Config.CoverSize = strings.ToLower(*overrides.CoverSize)
	}
	if overrides.CoverFormat != nil {
		Config.CoverFormat = strings.ToLower(*overrides.CoverFormat)
	}
	if overrides.EmbedLyrics != nil {
		Config.EmbedLrc = *overrides.EmbedLyrics
	}
	if overrides.SaveLyricsFile != nil {
		Config.SaveLrcFile = *overrides.SaveLyricsFile
	}
	if overrides.LyricsFormat != nil {
		Config.LrcFormat = strings.ToLower(*overrides.LyricsFormat)
	}
	if overrides.AacType != nil {
		Config.AacType = strings.ToLower(*overrides.AacType)
	}
	if overrides.AlacMax != nil {
		Config.AlacMax = *overrides.AlacMax
	}
	if overrides.AtmosMax != nil {
		Config.AtmosMax = *overrides.AtmosMax
	}
	return nil
}

func validateSafeOverrides(value *SafeOverrides) error {
	if value.Storefront != nil && !regexp.MustCompile(`^[A-Za-z]{2}$`).MatchString(*value.Storefront) {
		return fmt.Errorf("storefront must be a two-letter code")
	}
	for name, candidate := range map[string]*string{
		"alacSaveFolder":  value.AlacSaveFolder,
		"atmosSaveFolder": value.AtmosSaveFolder,
		"aacSaveFolder":   value.AacSaveFolder,
	} {
		if candidate != nil {
			if strings.ContainsRune(*candidate, '\x00') || !filepath.IsAbs(*candidate) {
				return fmt.Errorf("%s must be an absolute path", name)
			}
		}
	}
	if value.CoverSize != nil {
		var width, height int
		if _, err := fmt.Sscanf(strings.ToLower(*value.CoverSize), "%dx%d", &width, &height); err != nil || width < 100 || height < 100 || width > 10000 || height > 10000 {
			return fmt.Errorf("coverSize must be WIDTHxHEIGHT between 100 and 10000")
		}
	}
	if value.CoverFormat != nil && !oneOf(strings.ToLower(*value.CoverFormat), "jpg", "png", "original") {
		return fmt.Errorf("coverFormat must be jpg, png or original")
	}
	if value.LyricsFormat != nil && !oneOf(strings.ToLower(*value.LyricsFormat), "lrc", "ttml") {
		return fmt.Errorf("lyricsFormat must be lrc or ttml")
	}
	if value.AacType != nil && !oneOf(strings.ToLower(*value.AacType), "aac", "aac-lc", "aac-binaural", "aac-downmix") {
		return fmt.Errorf("aacType is not supported")
	}
	if value.AlacMax != nil && !oneOfInt(*value.AlacMax, 44100, 48000, 96000, 192000) {
		return fmt.Errorf("alacMax is not supported")
	}
	if value.AtmosMax != nil && !oneOfInt(*value.AtmosMax, 2448, 2768) {
		return fmt.Errorf("atmosMax is not supported")
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func oneOfInt(value int, allowed ...int) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func startMachineJob(urls []string) {
	if machineJobID == "" {
		machineJobID = fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	}
	emitMachineEvent("job_started", map[string]any{"urlCount": len(urls)})
}

func emitMachineEvent(eventType string, payload interface{}) {
	if !machine_events {
		return
	}
	event := machineEvent{
		Version:   1,
		Type:      eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		JobID:     machineJobID,
		Payload:   payload,
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return
	}
	machineEventMu.Lock()
	defer machineEventMu.Unlock()
	fmt.Printf("%s%s\n", machineEventPrefix, encoded)
}
