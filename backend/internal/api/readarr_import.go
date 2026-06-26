package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

const readarrImportTimeout = 30 * time.Second

type readarrImportRequest struct {
	BaseURL               string `json:"baseUrl"`
	APIKey                string `json:"apiKey"`
	ImportAuthors         *bool  `json:"importAuthors,omitempty"`
	ImportBooks           *bool  `json:"importBooks,omitempty"`
	ImportQualityProfiles *bool  `json:"importQualityProfiles,omitempty"`
	ImportRootFolders     *bool  `json:"importRootFolders,omitempty"`
	ImportTags            *bool  `json:"importTags,omitempty"`
	ImportLists           *bool  `json:"importLists,omitempty"`
	ImportListExclusions  *bool  `json:"importListExclusions,omitempty"`
}

type readarrImportOutcome struct {
	Status      string                 `json:"status"`
	DryRun      bool                   `json:"dryRun"`
	Source      string                 `json:"source"`
	Sections    []readarrImportSection `json:"sections"`
	Errors      []string               `json:"errors,omitempty"`
	GeneratedAt time.Time              `json:"generatedAt"`
}

type readarrImportSection struct {
	Name     string              `json:"name"`
	Count    int                 `json:"count"`
	Imported int                 `json:"imported"`
	Skipped  int                 `json:"skipped"`
	Errors   []string            `json:"errors,omitempty"`
	Items    []readarrImportItem `json:"items,omitempty"`
}

type readarrImportItem struct {
	ID             string `json:"id,omitempty"`
	Title          string `json:"title,omitempty"`
	AuthorName     string `json:"authorName,omitempty"`
	Path           string `json:"path,omitempty"`
	QualityProfile string `json:"qualityProfile,omitempty"`
	Status         string `json:"status,omitempty"`
	Message        string `json:"message,omitempty"`
}

type readarrAPIClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type readarrRemoteQualityProfile struct {
	ID                int                         `json:"id"`
	Name              string                      `json:"name"`
	UpgradeAllowed    bool                        `json:"upgradeAllowed"`
	MinFormatScore    float64                     `json:"minFormatScore"`
	CutoffFormatScore float64                     `json:"cutoffFormatScore"`
	Items             []readarrRemoteQualityItem  `json:"items"`
	FormatItems       []readarrRemoteQualityItem  `json:"formatItems"`
	Quality           *readarrRemoteQualityRecord `json:"quality"`
}

type readarrRemoteQualityItem struct {
	Allowed bool                        `json:"allowed"`
	Name    string                      `json:"name"`
	Quality *readarrRemoteQualityRecord `json:"quality"`
	Items   []readarrRemoteQualityItem  `json:"items"`
}

type readarrRemoteQualityRecord struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type readarrRemoteRootFolder struct {
	ID                      int    `json:"id"`
	Name                    string `json:"name"`
	Path                    string `json:"path"`
	DefaultQualityProfileID int    `json:"defaultQualityProfileId"`
}

type readarrRemoteAuthor struct {
	ID                int                  `json:"id"`
	AuthorName        string               `json:"authorName"`
	ForeignAuthorID   string               `json:"foreignAuthorId"`
	Path              string               `json:"path"`
	Monitored         bool                 `json:"monitored"`
	QualityProfileID  int                  `json:"qualityProfileId"`
	MetadataProfileID int                  `json:"metadataProfileId"`
	AddOptions        map[string]any       `json:"addOptions"`
	Images            []readarrRemoteImage `json:"images"`
	Tags              []int                `json:"tags"`
}

type readarrRemoteBook struct {
	ID               int                    `json:"id"`
	Title            string                 `json:"title"`
	ForeignBookID    string                 `json:"foreignBookId"`
	AuthorTitle      string                 `json:"authorTitle"`
	Monitored        bool                   `json:"monitored"`
	QualityProfileID int                    `json:"qualityProfileId"`
	Author           *readarrRemoteAuthor   `json:"author"`
	Images           []readarrRemoteImage   `json:"images"`
	Editions         []readarrRemoteEdition `json:"editions"`
	Tags             []int                  `json:"tags"`
}

type readarrRemoteEdition struct {
	ID               int                  `json:"id"`
	Title            string               `json:"title"`
	ForeignEditionID string               `json:"foreignEditionId"`
	Isbn13           string               `json:"isbn13"`
	Isbn             string               `json:"isbn"`
	ASIN             string               `json:"asin"`
	Format           string               `json:"format"`
	Language         string               `json:"language"`
	Monitored        bool                 `json:"monitored"`
	Images           []readarrRemoteImage `json:"images"`
}

type readarrRemoteImage struct {
	CoverType string `json:"coverType"`
	URL       string `json:"url"`
	RemoteURL string `json:"remoteUrl"`
}

func (h *handler) previewReadarrImport(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeReadarrImportRequest(w, r)
	if !ok {
		return
	}
	outcome, err := h.runReadarrImport(r.Context(), request, true)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) applyReadarrImport(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeReadarrImportRequest(w, r)
	if !ok {
		return
	}
	outcome, err := h.runReadarrImport(r.Context(), request, false)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func decodeReadarrImportRequest(w http.ResponseWriter, r *http.Request) (readarrImportRequest, bool) {
	defer r.Body.Close()
	var request readarrImportRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid Readarr import payload"})
		return readarrImportRequest{}, false
	}
	request.BaseURL = strings.TrimSpace(request.BaseURL)
	request.APIKey = strings.TrimSpace(request.APIKey)
	if request.BaseURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Readarr URL is required"})
		return readarrImportRequest{}, false
	}
	parsed, err := url.Parse(request.BaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Readarr URL must include a scheme and host"})
		return readarrImportRequest{}, false
	}
	return request, true
}

func (h *handler) runReadarrImport(ctx context.Context, request readarrImportRequest, dryRun bool) (readarrImportOutcome, error) {
	client := readarrAPIClient{
		baseURL:    strings.TrimRight(request.BaseURL, "/"),
		apiKey:     request.APIKey,
		httpClient: &http.Client{Timeout: readarrImportTimeout},
	}
	outcome := readarrImportOutcome{
		Status:      "ok",
		DryRun:      dryRun,
		Source:      request.BaseURL,
		GeneratedAt: time.Now().UTC(),
	}

	qualityProfileMap := map[int]string{}
	var profiles []readarrRemoteQualityProfile
	needsProfiles := readarrImportEnabled(request.ImportQualityProfiles) || readarrImportEnabled(request.ImportAuthors) || readarrImportEnabled(request.ImportBooks)
	if needsProfiles {
		if err := client.get(ctx, "/api/v1/qualityprofile", &profiles); err != nil {
			return outcome, fmt.Errorf("readarr quality profile fetch failed: %w", err)
		}
		for _, profile := range profiles {
			if strings.TrimSpace(profile.Name) != "" {
				qualityProfileMap[profile.ID] = strings.TrimSpace(profile.Name)
			}
		}
		if readarrImportEnabled(request.ImportQualityProfiles) {
			outcome.Sections = append(outcome.Sections, h.importReadarrQualityProfiles(ctx, profiles, dryRun))
		}
	}

	if readarrImportEnabled(request.ImportTags) {
		var tags []map[string]any
		if err := client.get(ctx, "/api/v1/tag", &tags); err != nil {
			return outcome, fmt.Errorf("readarr tag fetch failed: %w", err)
		}
		outcome.Sections = append(outcome.Sections, h.importReadarrCompatResources(ctx, "tags", "tag", tags, dryRun))
	}

	if readarrImportEnabled(request.ImportRootFolders) {
		var roots []readarrRemoteRootFolder
		if err := client.get(ctx, "/api/v1/rootfolder", &roots); err != nil {
			return outcome, fmt.Errorf("readarr root folder fetch failed: %w", err)
		}
		outcome.Sections = append(outcome.Sections, h.importReadarrRootFolders(ctx, roots, qualityProfileMap, dryRun))
	}

	if readarrImportEnabled(request.ImportAuthors) {
		var authors []readarrRemoteAuthor
		if err := client.get(ctx, "/api/v1/author", &authors); err != nil {
			return outcome, fmt.Errorf("readarr author fetch failed: %w", err)
		}
		outcome.Sections = append(outcome.Sections, h.importReadarrAuthors(ctx, authors, qualityProfileMap, dryRun))
	}

	if readarrImportEnabled(request.ImportBooks) {
		var books []readarrRemoteBook
		if err := client.get(ctx, "/api/v1/book", &books); err != nil {
			return outcome, fmt.Errorf("readarr book fetch failed: %w", err)
		}
		outcome.Sections = append(outcome.Sections, h.importReadarrBooks(ctx, books, qualityProfileMap, dryRun))
	}

	if readarrImportEnabled(request.ImportLists) {
		var lists []map[string]any
		if err := client.get(ctx, "/api/v1/importlist", &lists); err != nil {
			return outcome, fmt.Errorf("readarr import list fetch failed: %w", err)
		}
		outcome.Sections = append(outcome.Sections, h.importReadarrCompatResources(ctx, "importLists", "import-list", lists, dryRun))
	}

	if readarrImportEnabled(request.ImportListExclusions) {
		var exclusions []map[string]any
		if err := client.get(ctx, "/api/v1/importlistexclusion", &exclusions); err != nil {
			return outcome, fmt.Errorf("readarr import list exclusion fetch failed: %w", err)
		}
		outcome.Sections = append(outcome.Sections, h.importReadarrCompatResources(ctx, "importListExclusions", "import-list-exclusion", exclusions, dryRun))
	}

	for _, section := range outcome.Sections {
		outcome.Errors = append(outcome.Errors, section.Errors...)
	}
	if len(outcome.Errors) > 0 {
		outcome.Status = "partial"
	}
	return outcome, nil
}

func (client readarrAPIClient) get(ctx context.Context, path string, target any) error {
	endpoint := client.baseURL + path
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	if client.apiKey != "" {
		request.Header.Set("X-Api-Key", client.apiKey)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%s returned %d", path, response.StatusCode)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return err
	}
	return nil
}

func (h *handler) importReadarrQualityProfiles(ctx context.Context, profiles []readarrRemoteQualityProfile, dryRun bool) readarrImportSection {
	section := readarrImportSection{Name: "qualityProfiles", Count: len(profiles)}
	if h.deps.Wanted == nil && !dryRun {
		section.Errors = append(section.Errors, "wanted service is unavailable")
		section.Skipped = len(profiles)
	}
	for _, profile := range profiles {
		name := strings.TrimSpace(profile.Name)
		if name == "" {
			section.Skipped++
			section.Items = append(section.Items, readarrImportItem{ID: strconv.Itoa(profile.ID), Status: "skipped", Message: "missing profile name"})
			continue
		}
		item := readarrImportItem{
			ID:             strconv.Itoa(profile.ID),
			Title:          name,
			QualityProfile: name,
			Status:         "preview",
		}
		if dryRun || h.deps.Wanted == nil {
			section.Items = append(section.Items, item)
			continue
		}
		saved, err := h.deps.Wanted.SaveQualityProfile(ctx, wanted.QualityProfile{
			Name:           name,
			MediaFormat:    readarrQualityProfileFormat(profile),
			MinScore:       profile.MinFormatScore,
			CutoffScore:    profile.CutoffFormatScore,
			MinSeeders:     1,
			MaxSizeBytes:   readarrQualityProfileMaxSize(profile),
			PreferredTerms: readarrQualityProfilePreferredTerms(profile),
			RejectedTerms:  []string{"sample", "preview"},
			PreferredScore: 10,
			UpgradeAllowed: profile.UpgradeAllowed,
		})
		if err != nil {
			section.Errors = append(section.Errors, fmt.Sprintf("%s: %v", name, err))
			item.Status = "error"
			item.Message = err.Error()
			section.Skipped++
		} else {
			item.Status = "imported"
			item.ID = firstNonEmptyString(saved.ID, item.ID)
			section.Imported++
		}
		section.Items = append(section.Items, item)
	}
	return trimReadarrImportSection(section)
}

func (h *handler) importReadarrCompatResources(ctx context.Context, sectionName string, resourceType string, records []map[string]any, dryRun bool) readarrImportSection {
	section := readarrImportSection{Name: sectionName, Count: len(records)}
	if h.deps.Compat == nil && !dryRun {
		section.Errors = append(section.Errors, "compat resource store is unavailable")
		section.Skipped = len(records)
	}
	for _, record := range records {
		payload := cloneCompatRecord(record)
		payload["librarryImportedFrom"] = "readarr"
		compatID := stablePayloadID(payload, resourceType)
		name := firstNonEmptyString(compatResourceName(payload), resourceType)
		item := readarrImportItem{
			ID:      strconv.Itoa(compatID),
			Title:   name,
			Status:  "preview",
			Message: resourceType,
		}
		if dryRun || h.deps.Compat == nil {
			section.Items = append(section.Items, item)
			continue
		}
		saved, err := h.deps.Compat.UpsertResource(ctx, compatdata.Resource{
			ResourceType: resourceType,
			CompatID:     compatID,
			Name:         name,
			Payload:      payload,
		})
		if err != nil {
			section.Errors = append(section.Errors, fmt.Sprintf("%s: %v", name, err))
			item.Status = "error"
			item.Message = err.Error()
			section.Skipped++
		} else {
			item.Status = "imported"
			item.ID = strconv.Itoa(saved.CompatID)
			section.Imported++
		}
		section.Items = append(section.Items, item)
	}
	return trimReadarrImportSection(section)
}

func (h *handler) importReadarrRootFolders(ctx context.Context, roots []readarrRemoteRootFolder, qualityProfiles map[int]string, dryRun bool) readarrImportSection {
	section := readarrImportSection{Name: "rootFolders", Count: len(roots)}
	if h.deps.Compat == nil && !dryRun {
		section.Errors = append(section.Errors, "compat resource store is unavailable")
		section.Skipped = len(roots)
	}
	for _, root := range roots {
		path := strings.TrimSpace(root.Path)
		item := readarrImportItem{
			ID:             strconv.Itoa(root.ID),
			Title:          firstNonEmptyString(strings.TrimSpace(root.Name), readarrRootFolderName(path)),
			Path:           path,
			QualityProfile: qualityProfiles[root.DefaultQualityProfileID],
			Status:         "preview",
		}
		if path == "" {
			item.Status = "skipped"
			item.Message = "missing root folder path"
			section.Skipped++
			section.Items = append(section.Items, item)
			continue
		}
		if dryRun || h.deps.Compat == nil {
			section.Items = append(section.Items, item)
			continue
		}
		saved, err := h.deps.Compat.CreateRootFolder(ctx, compatdata.RootFolder{
			Name:        item.Title,
			Path:        path,
			MediaFormat: "ebook",
			Metadata: map[string]any{
				"source":                  "readarr-import",
				"readarrId":               root.ID,
				"qualityProfile":          qualityProfiles[root.DefaultQualityProfileID],
				"defaultQualityProfileId": root.DefaultQualityProfileID,
			},
		})
		if err != nil {
			section.Errors = append(section.Errors, fmt.Sprintf("%s: %v", path, err))
			item.Status = "error"
			item.Message = err.Error()
			section.Skipped++
		} else {
			item.Status = "imported"
			item.ID = firstNonEmptyString(saved.ID, item.ID)
			section.Imported++
		}
		section.Items = append(section.Items, item)
	}
	return trimReadarrImportSection(section)
}

func (h *handler) importReadarrAuthors(ctx context.Context, authors []readarrRemoteAuthor, qualityProfiles map[int]string, dryRun bool) readarrImportSection {
	section := readarrImportSection{Name: "authors", Count: len(authors)}
	if h.deps.Wanted == nil && !dryRun {
		section.Errors = append(section.Errors, "wanted service is unavailable")
		section.Skipped = len(authors)
	}
	for _, author := range authors {
		authorName := strings.TrimSpace(author.AuthorName)
		item := readarrImportItem{
			ID:             firstNonEmptyString(author.ForeignAuthorID, readarrIntID(author.ID), authorName),
			AuthorName:     authorName,
			Path:           strings.TrimSpace(author.Path),
			QualityProfile: readarrQualityProfileName(author.QualityProfileID, qualityProfiles),
			Status:         "preview",
		}
		if authorName == "" {
			item.Status = "skipped"
			item.Message = "missing author name"
			section.Skipped++
			section.Items = append(section.Items, item)
			continue
		}
		if dryRun || h.deps.Wanted == nil {
			section.Items = append(section.Items, item)
			continue
		}
		monitorNewItems := author.Monitored
		subscription, err := h.deps.Wanted.SubscribeAuthor(ctx, wanted.AuthorSubscribeRequest{
			Result:            readarrAuthorSearchResult(author),
			AuthorName:        authorName,
			Provider:          "Readarr",
			ProviderKey:       item.ID,
			Format:            "ebook",
			QualityProfile:    item.QualityProfile,
			MonitorNewItems:   &monitorNewItems,
			MissingBookPolicy: "all",
			Tags:              compactCompatTags(author.Tags),
		})
		if err != nil {
			section.Errors = append(section.Errors, fmt.Sprintf("%s: %v", authorName, err))
			item.Status = "error"
			item.Message = err.Error()
			section.Skipped++
		} else {
			item.Status = "imported"
			item.ID = firstNonEmptyString(subscription.ID, item.ID)
			section.Imported++
		}
		section.Items = append(section.Items, item)
	}
	return trimReadarrImportSection(section)
}

func (h *handler) importReadarrBooks(ctx context.Context, books []readarrRemoteBook, qualityProfiles map[int]string, dryRun bool) readarrImportSection {
	section := readarrImportSection{Name: "books", Count: len(books)}
	if h.deps.Wanted == nil && !dryRun {
		section.Errors = append(section.Errors, "wanted service is unavailable")
		section.Skipped = len(books)
	}
	for _, book := range books {
		title := strings.TrimSpace(book.Title)
		authorName := readarrBookAuthorName(book)
		item := readarrImportItem{
			ID:             firstNonEmptyString(book.ForeignBookID, readarrIntID(book.ID), title),
			Title:          title,
			AuthorName:     authorName,
			QualityProfile: readarrQualityProfileName(book.QualityProfileID, qualityProfiles),
			Status:         "preview",
		}
		if title == "" {
			item.Status = "skipped"
			item.Message = "missing book title"
			section.Skipped++
			section.Items = append(section.Items, item)
			continue
		}
		if dryRun || h.deps.Wanted == nil {
			section.Items = append(section.Items, item)
			continue
		}
		bookFormat := readarrBookFormat(book)
		wantedItem, err := h.deps.Wanted.Create(ctx, wanted.CreateRequest{
			Result:         readarrBookSearchResult(book, bookFormat),
			Format:         string(bookFormat),
			QualityProfile: item.QualityProfile,
			Tags:           compactCompatTags(book.Tags),
		})
		if err != nil {
			section.Errors = append(section.Errors, fmt.Sprintf("%s: %v", title, err))
			item.Status = "error"
			item.Message = err.Error()
			section.Skipped++
		} else {
			item.Status = "imported"
			item.ID = firstNonEmptyString(wantedItem.ID, item.ID)
			section.Imported++
		}
		section.Items = append(section.Items, item)
	}
	return trimReadarrImportSection(section)
}

func readarrImportEnabled(value *bool) bool {
	return value == nil || *value
}

func readarrRootFolderName(path string) string {
	name := filepath.Base(strings.TrimRight(strings.TrimSpace(path), "/"))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "Books"
	}
	return name
}

func readarrIntID(id int) string {
	if id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}

func readarrQualityProfileName(id int, profiles map[int]string) string {
	if name := strings.TrimSpace(profiles[id]); name != "" {
		return name
	}
	return "standard"
}

func readarrQualityProfileFormat(profile readarrRemoteQualityProfile) string {
	terms := strings.ToLower(strings.Join(append(readarrQualityProfilePreferredTerms(profile), profile.Name), " "))
	if strings.Contains(terms, "audio") || strings.Contains(terms, "m4b") || strings.Contains(terms, "mp3") {
		return "audiobook"
	}
	return "ebook"
}

func readarrQualityProfileMaxSize(profile readarrRemoteQualityProfile) int64 {
	if readarrQualityProfileFormat(profile) == "audiobook" {
		return 8 * 1024 * 1024 * 1024
	}
	return 2 * 1024 * 1024 * 1024
}

func readarrQualityProfilePreferredTerms(profile readarrRemoteQualityProfile) []string {
	terms := map[string]struct{}{}
	collectReadarrQualityTerms(profile.Items, terms)
	collectReadarrQualityTerms(profile.FormatItems, terms)
	if profile.Quality != nil {
		addReadarrQualityTerm(profile.Quality.Name, terms)
	}
	values := make([]string, 0, len(terms))
	for term := range terms {
		values = append(values, term)
	}
	sort.Strings(values)
	return values
}

func collectReadarrQualityTerms(items []readarrRemoteQualityItem, terms map[string]struct{}) {
	for _, item := range items {
		if item.Allowed {
			addReadarrQualityTerm(item.Name, terms)
			if item.Quality != nil {
				addReadarrQualityTerm(item.Quality.Name, terms)
			}
		}
		collectReadarrQualityTerms(item.Items, terms)
	}
}

func addReadarrQualityTerm(value string, terms map[string]struct{}) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "unknown" {
		return
	}
	terms[value] = struct{}{}
}

func readarrAuthorSearchResult(author readarrRemoteAuthor) metadata.SearchResult {
	name := strings.TrimSpace(author.AuthorName)
	key := firstNonEmptyString(author.ForeignAuthorID, readarrIntID(author.ID), name)
	return metadata.SearchResult{
		Provider: "Readarr",
		Kind:     metadata.SearchTypeAuthor,
		Work: metadata.Work{
			ID:    "readarr:author:" + key,
			Title: name,
			Authors: []metadata.Author{{
				ID:   "readarr:author:" + key,
				Name: name,
			}},
			CoverURL: readarrCoverURL(author.Images),
			ProviderIDs: []string{
				"readarr:" + key,
			},
		},
		Score:        100,
		Confidence:   "high",
		MatchedOn:    []string{"readarr-import"},
		RawSourceKey: key,
	}
}

func readarrBookSearchResult(book readarrRemoteBook, format metadata.MediaFormat) metadata.SearchResult {
	key := firstNonEmptyString(book.ForeignBookID, readarrIntID(book.ID), strings.TrimSpace(book.Title))
	edition := readarrPreferredEdition(book)
	editionKey := firstNonEmptyString(edition.ForeignEditionID, readarrIntID(edition.ID), key)
	authorName := readarrBookAuthorName(book)
	authorID := ""
	if book.Author != nil {
		authorID = firstNonEmptyString(book.Author.ForeignAuthorID, readarrIntID(book.Author.ID))
	}
	result := metadata.SearchResult{
		Provider: "Readarr",
		Kind:     metadata.SearchTypeBook,
		Work: metadata.Work{
			ID:       "readarr:book:" + key,
			Title:    strings.TrimSpace(book.Title),
			CoverURL: firstNonEmptyString(readarrCoverURL(book.Images), readarrCoverURL(edition.Images)),
			ProviderIDs: []string{
				"readarr:" + key,
			},
		},
		Edition: metadata.Edition{
			ID:       "readarr:edition:" + editionKey,
			WorkID:   "readarr:book:" + key,
			Title:    firstNonEmptyString(strings.TrimSpace(edition.Title), strings.TrimSpace(book.Title)),
			Format:   format,
			Language: strings.TrimSpace(edition.Language),
			ISBNs:    readarrEditionISBNs(edition),
			ASIN:     strings.TrimSpace(edition.ASIN),
		},
		Score:        100,
		Confidence:   "high",
		MatchedOn:    []string{"readarr-import"},
		RawSourceKey: key,
	}
	if authorName != "" {
		authorProviderID := "readarr:author:" + authorName
		if authorID != "" {
			authorProviderID = "readarr:author:" + authorID
		}
		result.Work.Authors = []metadata.Author{{
			ID:   authorProviderID,
			Name: authorName,
		}}
	}
	return result
}

func readarrPreferredEdition(book readarrRemoteBook) readarrRemoteEdition {
	for _, edition := range book.Editions {
		if edition.Monitored {
			return edition
		}
	}
	if len(book.Editions) > 0 {
		return book.Editions[0]
	}
	return readarrRemoteEdition{}
}

func readarrEditionISBNs(edition readarrRemoteEdition) []string {
	var values []string
	for _, value := range []string{edition.Isbn13, edition.Isbn} {
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func readarrBookAuthorName(book readarrRemoteBook) string {
	if book.Author != nil && strings.TrimSpace(book.Author.AuthorName) != "" {
		return strings.TrimSpace(book.Author.AuthorName)
	}
	return strings.TrimSpace(book.AuthorTitle)
}

func readarrBookFormat(book readarrRemoteBook) metadata.MediaFormat {
	for _, edition := range book.Editions {
		value := strings.ToLower(edition.Format + " " + edition.Title)
		if strings.Contains(value, "audio") || strings.Contains(value, "m4b") || strings.Contains(value, "mp3") {
			return metadata.FormatAudiobook
		}
	}
	return metadata.FormatEbook
}

func readarrCoverURL(images []readarrRemoteImage) string {
	for _, image := range images {
		if strings.EqualFold(image.CoverType, "cover") {
			return firstNonEmptyString(strings.TrimSpace(image.RemoteURL), strings.TrimSpace(image.URL))
		}
	}
	for _, image := range images {
		if url := firstNonEmptyString(strings.TrimSpace(image.RemoteURL), strings.TrimSpace(image.URL)); url != "" {
			return url
		}
	}
	return ""
}

func trimReadarrImportSection(section readarrImportSection) readarrImportSection {
	const maxItems = 12
	if len(section.Items) > maxItems {
		section.Items = section.Items[:maxItems]
	}
	return section
}
