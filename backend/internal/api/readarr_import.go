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
	"github.com/bandoracer/librarry/backend/internal/library"
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
	ImportBookFiles       *bool  `json:"importBookFiles,omitempty"`
	ImportTags            *bool  `json:"importTags,omitempty"`
	ImportLists           *bool  `json:"importLists,omitempty"`
	ImportListExclusions  *bool  `json:"importListExclusions,omitempty"`
	ImportConfigResources *bool  `json:"importConfigResources,omitempty"`
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

type readarrCompatResourceSpec struct {
	Endpoint     string
	SectionName  string
	ResourceType string
}

var readarrConfigResourceSpecs = []readarrCompatResourceSpec{
	{Endpoint: "/api/v1/delayprofile", SectionName: "delayProfiles", ResourceType: "delay-profile"},
	{Endpoint: "/api/v1/languageprofile", SectionName: "languageProfiles", ResourceType: "language-profile"},
	{Endpoint: "/api/v1/metadataprofile", SectionName: "metadataProfiles", ResourceType: "metadata-profile"},
	{Endpoint: "/api/v1/metadata", SectionName: "metadataConsumers", ResourceType: "metadata-consumer"},
	{Endpoint: "/api/v1/customformat", SectionName: "customFormats", ResourceType: "custom-format"},
	{Endpoint: "/api/v1/restriction", SectionName: "restrictions", ResourceType: "restriction"},
	{Endpoint: "/api/v1/notification", SectionName: "notifications", ResourceType: "notification"},
	{Endpoint: "/api/v1/remotepathmapping", SectionName: "remotePathMappings", ResourceType: "remote-path-mapping"},
	{Endpoint: "/api/v1/downloadclient", SectionName: "downloadClients", ResourceType: "download-client"},
	{Endpoint: "/api/v1/indexer", SectionName: "indexers", ResourceType: "indexer"},
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

type readarrRemoteBookFile struct {
	ID           int                   `json:"id"`
	AuthorID     int                   `json:"authorId"`
	BookID       int                   `json:"bookId"`
	EditionID    int                   `json:"editionId"`
	Path         string                `json:"path"`
	RelativePath string                `json:"relativePath"`
	Size         int64                 `json:"size"`
	DateAdded    string                `json:"dateAdded"`
	Modified     string                `json:"modified"`
	Quality      map[string]any        `json:"quality"`
	Languages    []map[string]any      `json:"languages"`
	Language     map[string]any        `json:"language"`
	MediaInfo    map[string]any        `json:"mediaInfo"`
	Book         *readarrRemoteBook    `json:"book"`
	Author       *readarrRemoteAuthor  `json:"author"`
	Edition      *readarrRemoteEdition `json:"edition"`
	ReleaseGroup string                `json:"releaseGroup"`
	SceneName    string                `json:"sceneName"`
	BookFileType string                `json:"bookFileType"`
	CalibreID    int                   `json:"calibreId"`
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
		h.appendReadarrCompatResourceImport(ctx, &outcome, client, readarrCompatResourceSpec{
			Endpoint: "/api/v1/tag", SectionName: "tags", ResourceType: "tag",
		}, dryRun)
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

	booksByID := map[int]readarrRemoteBook{}
	needsBooks := readarrImportEnabled(request.ImportBooks) || readarrImportEnabled(request.ImportBookFiles)
	if needsBooks {
		var books []readarrRemoteBook
		if err := client.get(ctx, "/api/v1/book", &books); err != nil {
			return outcome, fmt.Errorf("readarr book fetch failed: %w", err)
		}
		for _, book := range books {
			if book.ID > 0 {
				booksByID[book.ID] = book
			}
		}
		if readarrImportEnabled(request.ImportBooks) {
			outcome.Sections = append(outcome.Sections, h.importReadarrBooks(ctx, books, qualityProfileMap, dryRun))
		}
	}

	if readarrImportEnabled(request.ImportBookFiles) {
		var bookFiles []readarrRemoteBookFile
		if err := client.get(ctx, "/api/v1/bookfile", &bookFiles); err != nil {
			return outcome, fmt.Errorf("readarr bookfile fetch failed: %w", err)
		}
		outcome.Sections = append(outcome.Sections, h.importReadarrBookFiles(ctx, bookFiles, booksByID, dryRun))
	}

	if readarrImportEnabled(request.ImportLists) {
		h.appendReadarrCompatResourceImport(ctx, &outcome, client, readarrCompatResourceSpec{
			Endpoint: "/api/v1/importlist", SectionName: "importLists", ResourceType: "import-list",
		}, dryRun)
	}

	if readarrImportEnabled(request.ImportListExclusions) {
		h.appendReadarrCompatResourceImport(ctx, &outcome, client, readarrCompatResourceSpec{
			Endpoint: "/api/v1/importlistexclusion", SectionName: "importListExclusions", ResourceType: "import-list-exclusion",
		}, dryRun)
	}

	if readarrImportEnabled(request.ImportConfigResources) {
		for _, spec := range readarrConfigResourceSpecs {
			h.appendReadarrCompatResourceImport(ctx, &outcome, client, spec, dryRun)
		}
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

func (h *handler) appendReadarrCompatResourceImport(ctx context.Context, outcome *readarrImportOutcome, client readarrAPIClient, spec readarrCompatResourceSpec, dryRun bool) {
	var records []map[string]any
	if err := client.get(ctx, spec.Endpoint, &records); err != nil {
		outcome.Sections = append(outcome.Sections, readarrImportSection{
			Name:   spec.SectionName,
			Errors: []string{fmt.Sprintf("%s fetch failed: %v", spec.ResourceType, err)},
		})
		return
	}
	outcome.Sections = append(outcome.Sections, h.importReadarrCompatResources(ctx, spec.SectionName, spec.ResourceType, records, dryRun))
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
			Tags:              h.compatTagLabels(ctx, author.Tags),
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
			Tags:           h.compatTagLabels(ctx, book.Tags),
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

func (h *handler) importReadarrBookFiles(ctx context.Context, files []readarrRemoteBookFile, booksByID map[int]readarrRemoteBook, dryRun bool) readarrImportSection {
	section := readarrImportSection{Name: "bookFiles", Count: len(files)}
	if h.deps.Library == nil && !dryRun {
		section.Errors = append(section.Errors, "library service is unavailable")
		section.Skipped = len(files)
	}
	for _, remoteFile := range files {
		record := readarrBookFileRecord(remoteFile, booksByID)
		item := readarrImportItem{
			ID:         readarrIntID(remoteFile.ID),
			Title:      record.Title,
			AuthorName: record.AuthorName,
			Path:       record.Path,
			Status:     "preview",
		}
		if strings.TrimSpace(record.Path) == "" {
			item.Status = "skipped"
			item.Message = "missing file path"
			section.Skipped++
			section.Items = append(section.Items, item)
			continue
		}
		if dryRun || h.deps.Library == nil {
			section.Items = append(section.Items, item)
			continue
		}
		saved, err := h.deps.Library.TrackFile(ctx, record)
		if err != nil {
			section.Errors = append(section.Errors, fmt.Sprintf("%s: %v", record.Path, err))
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

func readarrBookFileRecord(remoteFile readarrRemoteBookFile, booksByID map[int]readarrRemoteBook) library.FileRecord {
	book := readarrBookForFile(remoteFile, booksByID)
	title := firstNonEmptyString(
		nestedString(map[string]any{"book": remoteBookPayload(book)}, "book", "title"),
		nestedString(map[string]any{"edition": remoteEditionPayload(remoteFile.Edition)}, "edition", "title"),
		strings.TrimSuffix(filepath.Base(remoteFile.Path), filepath.Ext(remoteFile.Path)),
	)
	authorName := readarrBookAuthorName(book)
	if remoteFile.Author != nil && strings.TrimSpace(remoteFile.Author.AuthorName) != "" {
		authorName = strings.TrimSpace(remoteFile.Author.AuthorName)
	}
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(remoteFile.Path)), ".")
	qualityName := readarrBookFileQualityName(remoteFile)
	mediaFormat := readarrBookFileFormat(remoteFile, book, extension, qualityName)
	metadataValues := map[string]any{
		"source":            "readarr-import",
		"readarrBookFileId": remoteFile.ID,
		"readarrBookId":     remoteFile.BookID,
		"readarrAuthorId":   remoteFile.AuthorID,
		"readarrEditionId":  remoteFile.EditionID,
		"qualityName":       qualityName,
		"languages":         readarrBookFileLanguages(remoteFile),
		"mediaInfo":         remoteFile.MediaInfo,
		"releaseGroup":      strings.TrimSpace(remoteFile.ReleaseGroup),
		"sceneName":         strings.TrimSpace(remoteFile.SceneName),
		"bookFileType":      strings.TrimSpace(remoteFile.BookFileType),
		"dateAdded":         strings.TrimSpace(remoteFile.DateAdded),
	}
	if remoteFile.CalibreID > 0 {
		metadataValues["calibreId"] = remoteFile.CalibreID
	}
	modifiedAt := readarrParseTime(firstNonEmptyString(remoteFile.Modified, remoteFile.DateAdded))
	return library.FileRecord{
		MediaFormat:  string(mediaFormat),
		Path:         strings.TrimSpace(remoteFile.Path),
		SourcePath:   strings.TrimSpace(remoteFile.Path),
		Title:        title,
		AuthorName:   authorName,
		Extension:    extension,
		SizeBytes:    remoteFile.Size,
		ImportStatus: "imported",
		Metadata:     metadataValues,
		ModifiedAt:   modifiedAt,
	}
}

func readarrBookForFile(remoteFile readarrRemoteBookFile, booksByID map[int]readarrRemoteBook) readarrRemoteBook {
	if remoteFile.Book != nil {
		return *remoteFile.Book
	}
	if book, ok := booksByID[remoteFile.BookID]; ok {
		return book
	}
	return readarrRemoteBook{}
}

func remoteBookPayload(book readarrRemoteBook) map[string]any {
	return map[string]any{"title": strings.TrimSpace(book.Title)}
}

func remoteEditionPayload(edition *readarrRemoteEdition) map[string]any {
	if edition == nil {
		return map[string]any{}
	}
	return map[string]any{"title": strings.TrimSpace(edition.Title)}
}

func readarrBookFileQualityName(remoteFile readarrRemoteBookFile) string {
	return firstNonEmptyString(
		nestedString(remoteFile.Quality, "quality", "name"),
		payloadString(remoteFile.Quality, "name"),
		payloadString(remoteFile.Quality, "qualityName"),
	)
}

func readarrBookFileLanguages(remoteFile readarrRemoteBookFile) []string {
	var languages []string
	for _, language := range remoteFile.Languages {
		if name := payloadString(language, "name"); name != "" {
			languages = append(languages, name)
		}
	}
	if name := payloadString(remoteFile.Language, "name"); name != "" {
		languages = append(languages, name)
	}
	if len(languages) == 0 {
		languages = append(languages, "English")
	}
	return firstUniqueStrings(languages)
}

func readarrBookFileFormat(remoteFile readarrRemoteBookFile, book readarrRemoteBook, extension string, qualityName string) metadata.MediaFormat {
	value := strings.ToLower(strings.Join([]string{
		remoteFile.BookFileType,
		extension,
		qualityName,
		string(readarrBookFormat(book)),
	}, " "))
	if strings.Contains(value, "audio") || strings.Contains(value, "m4b") || strings.Contains(value, "mp3") {
		return metadata.FormatAudiobook
	}
	return metadata.FormatEbook
}

func readarrParseTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			parsed = parsed.UTC()
			return &parsed
		}
	}
	return nil
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
