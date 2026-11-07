package upload

import (
	"strings"
	"testing"
)

// A13: the real type comes from the magic bytes; SVG, HTML and a renamed file are refused.
func TestSniff(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"pdf", "%PDF-1.7\n...", TypePDF},
		{"jpeg", "\xFF\xD8\xFF\xE0rest", TypeJPEG},
		{"png", "\x89PNG\r\n\x1a\nrest", TypePNG},
		{"svg", `<svg xmlns="http://www.w3.org/2000/svg"></svg>`, ""},
		{"html", "<!doctype html><script>", ""},
		{"truncated png header", "\x89PNG\r\n", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Sniff([]byte(tt.content)); got != tt.want {
				t.Fatalf("Sniff = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanFileName(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"estatuto.pdf", "estatuto.pdf"},
		{"../../etc/passwd", "passwd"},
		{`C:\Users\ana\dni frente.jpg`, "dni frente.jpg"},
		{"evil\x00name\n.png", "evilname.png"},
		{"", "document"},
		{"   ", "document"},
		{"/", "document"},
		{strings.Repeat("a", 200), strings.Repeat("a", 200)},
		{strings.Repeat("ñ", 201), strings.Repeat("ñ", 200)},
	}
	for _, tt := range tests {
		if got := CleanFileName(tt.in); got != tt.want {
			t.Errorf("CleanFileName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSHA256(t *testing.T) {
	t.Parallel()
	if got := SHA256([]byte("abc")); len(got) != 32 || got[0] != 0xba || got[31] != 0xad {
		t.Fatalf("SHA256(abc) = %x", got)
	}
}
