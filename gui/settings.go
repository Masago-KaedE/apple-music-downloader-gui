package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"
)

func defaultSettings() Settings {
	root := `C:\Apple-Music-Downloader`
	return Settings{
		ProjectRoot:     root,
		Distribution:    "Ubuntu",
		Storefront:      "us",
		AlacSaveFolder:  filepath.Join(root, "AM-DL downloads"),
		AtmosSaveFolder: filepath.Join(root, "AM-DL-Atmos downloads"),
		AacSaveFolder:   filepath.Join(root, "AM-DL-AAC downloads"),
		EmbedCover:      true,
		CoverSize:       "5000x5000",
		CoverFormat:     "jpg",
		EmbedLyrics:     true,
		SaveLyricsFile:  false,
		LyricsFormat:    "lrc",
		AacType:         "aac-lc",
		AlacMax:         192000,
		AtmosMax:        2768,
	}
}

func loadSettings(dir string) (Settings, error) {
	settings := defaultSettings()
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if os.IsNotExist(err) {
		return settings, nil
	}
	if err != nil {
		return Settings{}, err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, fmt.Errorf("解析设置失败: %w", err)
	}
	return validateSettings(settings)
}

func persistSettings(dir string, settings Settings) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("创建设置目录失败: %w", err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "settings-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	destination := filepath.Join(dir, "settings.json")
	_ = os.Remove(destination)
	if err := os.Rename(tmpName, destination); err != nil {
		return fmt.Errorf("保存设置失败: %w", err)
	}
	return nil
}

func validateSettings(settings Settings) (Settings, error) {
	settings.ProjectRoot = filepath.Clean(strings.TrimSpace(settings.ProjectRoot))
	if !filepath.IsAbs(settings.ProjectRoot) {
		return Settings{}, fmt.Errorf("项目目录必须是绝对路径")
	}
	settings.Distribution = strings.TrimSpace(settings.Distribution)
	if settings.Distribution == "" || !regexp.MustCompile(`^[\pL\pN._ -]+$`).MatchString(settings.Distribution) {
		return Settings{}, fmt.Errorf("WSL 发行版名称无效")
	}
	settings.Storefront = strings.ToLower(strings.TrimSpace(settings.Storefront))
	if !regexp.MustCompile(`^[a-z]{2}$`).MatchString(settings.Storefront) {
		return Settings{}, fmt.Errorf("Storefront 必须是两个英文字母")
	}
	for name, value := range map[string]string{
		"ALAC 输出目录":  settings.AlacSaveFolder,
		"Atmos 输出目录": settings.AtmosSaveFolder,
		"AAC 输出目录":   settings.AacSaveFolder,
	} {
		if !filepath.IsAbs(value) || strings.ContainsRune(value, '\x00') {
			return Settings{}, fmt.Errorf("%s必须是绝对路径", name)
		}
	}
	settings.CoverSize = strings.ToLower(strings.TrimSpace(settings.CoverSize))
	var width, height int
	if _, err := fmt.Sscanf(settings.CoverSize, "%dx%d", &width, &height); err != nil || width < 100 || height < 100 || width > 10000 || height > 10000 {
		return Settings{}, fmt.Errorf("封面尺寸必须为 100 到 10000 之间的 WIDTHxHEIGHT")
	}
	settings.CoverFormat = strings.ToLower(settings.CoverFormat)
	if !containsString([]string{"jpg", "png", "original"}, settings.CoverFormat) {
		return Settings{}, fmt.Errorf("不支持的封面格式")
	}
	settings.LyricsFormat = strings.ToLower(settings.LyricsFormat)
	if !containsString([]string{"lrc", "ttml"}, settings.LyricsFormat) {
		return Settings{}, fmt.Errorf("不支持的歌词格式")
	}
	settings.AacType = strings.ToLower(settings.AacType)
	if !containsString([]string{"aac", "aac-lc", "aac-binaural", "aac-downmix"}, settings.AacType) {
		return Settings{}, fmt.Errorf("不支持的 AAC 类型")
	}
	if !containsInt([]int{44100, 48000, 96000, 192000}, settings.AlacMax) {
		return Settings{}, fmt.Errorf("不支持的 ALAC 上限")
	}
	if !containsInt([]int{2448, 2768}, settings.AtmosMax) {
		return Settings{}, fmt.Errorf("不支持的 Atmos 上限")
	}
	return settings, nil
}

func inspectProject(root string) ProjectStatus {
	status := ProjectStatus{Root: root}
	status.Executable = isFile(filepath.Join(root, "apple-music-downloader.exe"))
	status.Config = isFile(filepath.Join(root, "config.yaml"))
	status.Wrapper = isFile(filepath.Join(root, "wrapper-runtime", "wrapper"))
	status.Valid = status.Executable && status.Config && status.Wrapper
	if status.Valid {
		status.Description = "项目组件完整"
	} else {
		missing := make([]string, 0, 3)
		if !status.Executable {
			missing = append(missing, "apple-music-downloader.exe")
		}
		if !status.Config {
			missing = append(missing, "config.yaml")
		}
		if !status.Wrapper {
			missing = append(missing, "wrapper-runtime/wrapper")
		}
		status.Description = "缺少：" + strings.Join(missing, "、")
	}
	return status
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func writeSafeOverrides(dir string, settings Settings) (string, error) {
	value := map[string]interface{}{
		"storefront":      settings.Storefront,
		"alacSaveFolder":  settings.AlacSaveFolder,
		"atmosSaveFolder": settings.AtmosSaveFolder,
		"aacSaveFolder":   settings.AacSaveFolder,
		"embedCover":      settings.EmbedCover,
		"coverSize":       settings.CoverSize,
		"coverFormat":     settings.CoverFormat,
		"embedLyrics":     settings.EmbedLyrics,
		"saveLyricsFile":  settings.SaveLyricsFile,
		"lyricsFormat":    settings.LyricsFormat,
		"aacType":         settings.AacType,
		"alacMax":         settings.AlacMax,
		"atmosMax":        settings.AtmosMax,
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "safe-overrides.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func listDistributions() []string {
	output, err := exec.Command("wsl.exe", "--list", "--quiet").Output()
	if err != nil {
		return nil
	}
	text := decodeWindowsCommandOutput(output)
	seen := map[string]bool{}
	var values []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "\x00\r"))
		if line != "" && !seen[line] {
			seen[line] = true
			values = append(values, line)
		}
	}
	sort.Strings(values)
	return values
}

func decodeWindowsCommandOutput(data []byte) string {
	if len(data) >= 2 && (data[1] == 0 || (data[0] == 0xff && data[1] == 0xfe)) {
		if data[0] == 0xff && data[1] == 0xfe {
			data = data[2:]
		}
		words := make([]uint16, 0, len(data)/2)
		for i := 0; i+1 < len(data); i += 2 {
			words = append(words, uint16(data[i])|uint16(data[i+1])<<8)
		}
		return string(utf16.Decode(words))
	}
	return string(data)
}

func windowsPathToWSL(path string) string {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	if len(volume) == 2 && volume[1] == ':' {
		rest := strings.TrimPrefix(clean, volume)
		rest = strings.ReplaceAll(rest, `\`, "/")
		return "/mnt/" + strings.ToLower(volume[:1]) + rest
	}
	return strings.ReplaceAll(clean, `\`, "/")
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func containsInt(values []int, candidate int) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
