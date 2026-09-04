package main

import (
	"regexp"
	"strings"
)

var emailPattern = regexp.MustCompile(`(?i)[a-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+`)

func redactLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	for _, marker := range []string{
		"music-token", "music_token", "media-user-token", "authorization-token",
		"authorization_token", "password", "passwd", "apple id", "appleid",
		"verification code", "2fa", "two-factor", "account info cached",
	} {
		if strings.Contains(lower, marker) {
			return "[敏感信息已隐藏]"
		}
	}
	return emailPattern.ReplaceAllString(trimmed, "[账号已隐藏]")
}
