package library

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRenderNamingTemplateWithAllTokens(t *testing.T) {
	values := map[string]string{
		"Author":         "Matt Dinniman",
		"Title":          "Dungeon Crawler Carl",
		"Series":         "Dungeon Crawler Carl",
		"SeriesPosition": "1",
		"Year":           "2020",
		"Format":         "ebook",
		"Ext":            ".epub",
	}
	got := RenderNamingTemplate("{Author} - {Series} #{SeriesPosition} - {Title} ({Year})", values)
	want := "Matt Dinniman - Dungeon Crawler Carl #1 - Dungeon Crawler Carl (2020)"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRenderNamingTemplateCollapsesEmptyTokens(t *testing.T) {
	tests := []struct {
		name     string
		template string
		values   map[string]string
		want     string
	}{
		{
			name:     "missing series and year",
			template: "{Author} - {Series} #{SeriesPosition} - {Title} ({Year})",
			values:   map[string]string{"Author": "Andy Weir", "Title": "Project Hail Mary", "Series": "", "SeriesPosition": "", "Year": ""},
			want:     "Andy Weir - Project Hail Mary",
		},
		{
			name:     "missing year keeps series",
			template: "{Series} {SeriesPosition} - {Title} ({Year})",
			values:   map[string]string{"Title": "Carl", "Series": "DCC", "SeriesPosition": "2", "Year": ""},
			want:     "DCC 2 - Carl",
		},
		{
			name:     "leading empty token",
			template: "{Series} - {Title}",
			values:   map[string]string{"Title": "Carl", "Series": ""},
			want:     "Carl",
		},
		{
			name:     "trailing empty token",
			template: "{Title} - {Series}",
			values:   map[string]string{"Title": "Carl", "Series": ""},
			want:     "Carl",
		},
		{
			name:     "bracketed empty token",
			template: "{Title} [{Year}]",
			values:   map[string]string{"Title": "Carl", "Year": ""},
			want:     "Carl",
		},
		{
			name:     "unknown tokens stay literal",
			template: "{Title} {Unknown}",
			values:   map[string]string{"Title": "Carl"},
			want:     "Carl {Unknown}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RenderNamingTemplate(tt.template, tt.values); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestDestinationPathRendersSeriesTokens(t *testing.T) {
	service := NewService(nil, Config{
		EbookRoot:                  "/library/ebooks",
		NamingAuthorFolderTemplate: "{Author}",
		NamingBookFolderTemplate:   "{Series} {SeriesPosition} - {Title}",
		NamingFileNameTemplate:     "{Title} ({Year}){Ext}",
	}, nil, nil)
	root := service.importRootPath(context.Background(), "ebook", "")

	withSeries := service.destinationPathIn(root, "ebook", parsedBook{
		AuthorName:     "Matt Dinniman",
		Title:          "Dungeon Crawler Carl",
		Series:         "Dungeon Crawler Carl",
		SeriesPosition: "1",
		Year:           "2020",
	}, ".epub")
	want := filepath.Join("/library/ebooks", "Matt Dinniman", "Dungeon Crawler Carl 1 - Dungeon Crawler Carl", "Dungeon Crawler Carl (2020).epub")
	if withSeries != want {
		t.Fatalf("expected %s, got %s", want, withSeries)
	}

	withoutSeries := service.destinationPathIn(root, "ebook", parsedBook{
		AuthorName: "Andy Weir",
		Title:      "Project Hail Mary",
	}, ".epub")
	want = filepath.Join("/library/ebooks", "Andy Weir", "Project Hail Mary", "Project Hail Mary.epub")
	if withoutSeries != want {
		t.Fatalf("expected empty tokens to collapse, want %s got %s", want, withoutSeries)
	}
}

func TestYearFromString(t *testing.T) {
	tests := map[string]string{
		"2021":         "2021",
		"2021-03-04":   "2021",
		"March 2021":   "2021",
		"21":           "",
		"":             "",
		"0021":         "",
		"123456":       "",
		"isbn 9780593": "",
	}
	for input, want := range tests {
		if got := yearFromString(input); got != want {
			t.Fatalf("yearFromString(%q): expected %q, got %q", input, want, got)
		}
	}
}
