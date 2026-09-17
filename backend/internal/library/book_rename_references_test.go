package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBookRenameCompanionReferences(t *testing.T) {
	for _, tc := range []struct {
		name, extension, content string
		valid                    bool
	}{
		{"cue quoted", ".cue", "FILE \"Disc 1/chapter.mp3\" MP3\n TRACK 01 AUDIO\n", true},
		{"cue malformed extra directive", ".cue", "FILE \"Disc 1/chapter.mp3\" MP3\nFILE broken\n", false},
		{"cue absolute", ".cue", "FILE \"/Disc 1/chapter.mp3\" MP3\n", false},
		{"cue no files", ".cue", "TITLE Book\n", false},
		{"opf namespace", ".opf", `<package xmlns="http://www.idpf.org/2007/opf"><manifest><item href="Disc 1/chapter.mp3#start"/></manifest><guide><reference href="cover.jpg"/></guide></package>`, true},
		{"opf remote link", ".opf", `<package><manifest><item href="https://example.invalid/cover.jpg"/></manifest></package>`, true},
		{"opf missing", ".opf", `<package><manifest><item href="missing.mp3"/></manifest></package>`, false},
		{"opf escape", ".opf", `<package><guide><reference href="../outside.mp3"/></guide></package>`, false},
		{"opf malformed", ".opf", `<package><manifest>`, false},
		{"playlist BOM and CRLF", ".m3u8", "\ufeff#EXTM3U\r\nDisc 1/chapter.mp3\r\n", true},
		{"playlist windows separators", ".m3u", "Disc 1\\chapter.mp3\n", true},
		{"playlist drive", ".m3u", "C:\\Disc 1\\chapter.mp3\n", false},
		{"invalid utf8", ".m3u", "\xff", false},
		{"oversize", ".m3u", strings.Repeat("#", (4<<20)+1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "companion"+tc.extension)
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}
			files := map[string]ImportOperationFile{
				filepath.Join(root, "Disc 1", "chapter.mp3"): {},
				filepath.Join(root, "cover.jpg"):             {},
			}
			err := verifyBookRenameReferences(path, root, files)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}
