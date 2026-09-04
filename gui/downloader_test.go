package main

import "testing"

func TestValidateDownloadRequest(t *testing.T) {
	request := DownloadRequest{
		Quality: "alac",
		URLs: []string{
			"https://music.apple.com/us/album/example/123?i=456",
			"https://music.apple.com/us/album/example/123?i=456",
			"https://music.apple.com/us/playlist/example/pl.123",
		},
	}
	urls, err := validateDownloadRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 2 {
		t.Fatalf("expected duplicate removal, got %d URLs", len(urls))
	}
	if !isSingleSongURL(urls[0]) || isSingleSongURL(urls[1]) {
		t.Fatal("single-song detection failed")
	}
}

func TestValidateDownloadRequestRejectsUnsupportedInput(t *testing.T) {
	for _, request := range []DownloadRequest{
		{Quality: "unknown", URLs: []string{"https://music.apple.com/us/album/a/1"}},
		{Quality: "alac", URLs: []string{"https://example.com/us/album/a/1"}},
		{Quality: "alac", URLs: []string{"https://music.apple.com/us/music-video/a/1"}},
		{Quality: "alac", URLs: nil},
	} {
		if _, err := validateDownloadRequest(request); err == nil {
			t.Fatalf("expected rejection: %+v", request)
		}
	}
}

func TestRedactLine(t *testing.T) {
	tests := map[string]string{
		"Music-Token: abcdef":                    "[敏感信息已隐藏]",
		"password=secret":                        "[敏感信息已隐藏]",
		"logged in as person@example.com":        "logged in as [账号已隐藏]",
		"Downloading track 1":                    "Downloading track 1",
		"Please enter 2FA verification code: 12": "[敏感信息已隐藏]",
	}
	for input, want := range tests {
		if got := redactLine(input); got != want {
			t.Fatalf("redactLine(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBashQuote(t *testing.T) {
	if got := bashQuote("/mnt/c/It's Here"); got != "'/mnt/c/It'\\''s Here'" {
		t.Fatalf("unexpected quote: %s", got)
	}
}
