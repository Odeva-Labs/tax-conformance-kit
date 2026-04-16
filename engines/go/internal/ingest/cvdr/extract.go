package cvdr

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

type ExtractionRequest struct {
	SelectionDir      string
	IndexDir          string
	OutputDir         string
	IncludeHistorical bool
}

type ExtractionManifest struct {
	ExtractedAt           string          `json:"extracted_at"`
	SelectionDir          string          `json:"selection_dir"`
	IndexDir              string          `json:"index_dir"`
	IncludeHistorical     bool            `json:"include_historical"`
	WorkCount             int             `json:"work_count"`
	BundleCount           int             `json:"bundle_count"`
	CurrentBundleCount    int             `json:"current_bundle_count"`
	HistoricalBundleCount int             `json:"historical_bundle_count"`
	Works                 []ExtractedWork `json:"works,omitempty"`
}

type ExtractedWork struct {
	CVDRID              string            `json:"cvdr_id"`
	Creator             string            `json:"creator,omitempty"`
	RepresentativeTitle string            `json:"representative_title,omitempty"`
	Bundles             []ExtractedBundle `json:"bundles"`
}

type ExtractedBundle struct {
	Identifier         string   `json:"identifier"`
	SelectionRole      string   `json:"selection_role"`
	SelectionReasons   []string `json:"selection_reasons"`
	PreferredURL       string   `json:"preferred_url,omitempty"`
	PreferredWorkURL   string   `json:"preferred_work_url,omitempty"`
	PublicationXMLURL  string   `json:"publication_xml_url,omitempty"`
	Issued             string   `json:"issued,omitempty"`
	EffectiveFrom      string   `json:"effective_from,omitempty"`
	EffectiveTo        string   `json:"effective_to,omitempty"`
	ChangeNature       string   `json:"change_nature,omitempty"`
	ChangeCategory     string   `json:"change_category,omitempty"`
	BundleDirPath      string   `json:"bundle_dir_path"`
	RecordPath         string   `json:"record_path"`
	DraftPath          string   `json:"draft_path"`
	PublicationXMLPath string   `json:"publication_xml_path"`
}

type DraftStub struct {
	ArtifactType         string              `json:"artifact_type"`
	Status               string              `json:"status"`
	ExtractedAt          string              `json:"extracted_at"`
	Domain               string              `json:"domain"`
	SourceType           string              `json:"source_type"`
	Jurisdiction         DraftJurisdiction   `json:"jurisdiction"`
	Selection            DraftSelection      `json:"selection"`
	Source               DraftSource         `json:"source"`
	SuggestedFixturePath string              `json:"suggested_fixture_path"`
	TODO                 []string            `json:"todo"`
	Work                 DraftWorkDescriptor `json:"work"`
}

type DraftJurisdiction struct {
	CountryCode      string `json:"country_code"`
	CountryName      string `json:"country_name"`
	MunicipalityName string `json:"municipality_name,omitempty"`
}

type DraftSelection struct {
	Role    string   `json:"role"`
	Reasons []string `json:"reasons"`
}

type DraftSource struct {
	CVDRID            string `json:"cvdr_id"`
	Identifier        string `json:"identifier"`
	PreferredURL      string `json:"preferred_url,omitempty"`
	PreferredWorkURL  string `json:"preferred_work_url,omitempty"`
	PublicationXMLURL string `json:"publication_xml_url,omitempty"`
	Issued            string `json:"issued,omitempty"`
	EffectiveFrom     string `json:"effective_from,omitempty"`
	EffectiveTo       string `json:"effective_to,omitempty"`
	ChangeNature      string `json:"change_nature,omitempty"`
	ChangeCategory    string `json:"change_category,omitempty"`
	Title             string `json:"title,omitempty"`
	Alternative       string `json:"alternative,omitempty"`
}

type DraftWorkDescriptor struct {
	CVDRID              string `json:"cvdr_id"`
	Creator             string `json:"creator,omitempty"`
	RepresentativeTitle string `json:"representative_title,omitempty"`
}

type SelectionExportManifest struct {
	SelectedAt               string `json:"selected_at"`
	IndexDir                 string `json:"index_dir"`
	AsOfDate                 string `json:"as_of_date"`
	MaxHistoricalPerWork     int    `json:"max_historical_per_work"`
	WorkCount                int    `json:"work_count"`
	CurrentCandidateCount    int    `json:"current_candidate_count"`
	HistoricalCandidateCount int    `json:"historical_candidate_count"`
}

func ExtractStubs(req ExtractionRequest, client Client) (ExtractionManifest, error) {
	if req.SelectionDir == "" {
		return ExtractionManifest{}, fmt.Errorf("selection directory is required")
	}
	if req.OutputDir == "" {
		return ExtractionManifest{}, fmt.Errorf("output directory is required")
	}

	selectionManifest, err := readSelectionExportManifest(filepath.Join(req.SelectionDir, "manifest.json"))
	if err != nil {
		return ExtractionManifest{}, err
	}
	if req.IndexDir == "" {
		req.IndexDir = selectionManifest.IndexDir
	}
	if req.IndexDir == "" {
		return ExtractionManifest{}, fmt.Errorf("index directory is required")
	}

	selections, err := readSelectionWorkFiles(req.SelectionDir)
	if err != nil {
		return ExtractionManifest{}, err
	}
	indexFiles, err := readWorkFiles(req.IndexDir)
	if err != nil {
		return ExtractionManifest{}, err
	}
	indexByWork := make(map[string]WorkFile, len(indexFiles))
	for _, workFile := range indexFiles {
		indexByWork[workFile.Work.CVDRID] = workFile
	}

	if err := os.MkdirAll(filepath.Join(req.OutputDir, "works"), 0o755); err != nil {
		return ExtractionManifest{}, err
	}
	if err := os.MkdirAll(filepath.Join(req.OutputDir, "bundles"), 0o755); err != nil {
		return ExtractionManifest{}, err
	}

	manifest := ExtractionManifest{
		ExtractedAt:       time.Now().UTC().Format(time.RFC3339),
		SelectionDir:      req.SelectionDir,
		IndexDir:          req.IndexDir,
		IncludeHistorical: req.IncludeHistorical,
		Works:             make([]ExtractedWork, 0, len(selections)),
	}

	for _, selection := range selections {
		workFile, ok := indexByWork[selection.CVDRID]
		if !ok {
			return ExtractionManifest{}, fmt.Errorf("missing harvested work file for %s", selection.CVDRID)
		}
		recordByIdentifier := make(map[string]Record, len(workFile.Versions))
		for _, record := range workFile.Versions {
			recordByIdentifier[record.Identifier] = record
		}

		workOut := ExtractedWork{
			CVDRID:              selection.CVDRID,
			Creator:             selection.Creator,
			RepresentativeTitle: selection.RepresentativeTitle,
			Bundles:             []ExtractedBundle{},
		}

		currentRecord, ok := recordByIdentifier[selection.CurrentCandidate.Identifier]
		if !ok {
			return ExtractionManifest{}, fmt.Errorf("missing selected record %s for work %s", selection.CurrentCandidate.Identifier, selection.CVDRID)
		}
		currentBundle, err := exportExtractedBundle(req.OutputDir, selection, workFile.Work, currentRecord, selection.CurrentCandidate, "current", client)
		if err != nil {
			return ExtractionManifest{}, err
		}
		workOut.Bundles = append(workOut.Bundles, currentBundle)
		manifest.CurrentBundleCount++

		if req.IncludeHistorical {
			for _, candidate := range selection.HistoricalCandidates {
				record, ok := recordByIdentifier[candidate.Identifier]
				if !ok {
					return ExtractionManifest{}, fmt.Errorf("missing historical record %s for work %s", candidate.Identifier, selection.CVDRID)
				}
				bundle, err := exportExtractedBundle(req.OutputDir, selection, workFile.Work, record, candidate, "historical", client)
				if err != nil {
					return ExtractionManifest{}, err
				}
				workOut.Bundles = append(workOut.Bundles, bundle)
				manifest.HistoricalBundleCount++
			}
		}

		sort.Slice(workOut.Bundles, func(i, j int) bool {
			if workOut.Bundles[i].SelectionRole == workOut.Bundles[j].SelectionRole {
				return workOut.Bundles[i].Identifier < workOut.Bundles[j].Identifier
			}
			return workOut.Bundles[i].SelectionRole < workOut.Bundles[j].SelectionRole
		})

		manifest.BundleCount += len(workOut.Bundles)
		manifest.Works = append(manifest.Works, workOut)
		if err := writeJSONFile(filepath.Join(req.OutputDir, "works", selection.CVDRID+".json"), workOut); err != nil {
			return ExtractionManifest{}, err
		}
	}

	sort.Slice(manifest.Works, func(i, j int) bool { return manifest.Works[i].CVDRID < manifest.Works[j].CVDRID })
	manifest.WorkCount = len(manifest.Works)

	workIndex := make([]ExtractedWork, 0, len(manifest.Works))
	for _, work := range manifest.Works {
		workIndex = append(workIndex, work)
	}
	if err := writeJSONFile(filepath.Join(req.OutputDir, "work_index.json"), workIndex); err != nil {
		return ExtractionManifest{}, err
	}

	manifestOut := manifest
	manifestOut.Works = nil
	if err := writeJSONFile(filepath.Join(req.OutputDir, "manifest.json"), manifestOut); err != nil {
		return ExtractionManifest{}, err
	}

	return manifest, nil
}

func exportExtractedBundle(outputDir string, selection WorkSelection, work WorkTimeline, record Record, candidate Candidate, selectionRole string, client Client) (ExtractedBundle, error) {
	bundleDir := filepath.Join(outputDir, "bundles", selection.CVDRID, record.Identifier)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		return ExtractedBundle{}, err
	}

	publicationXMLPath := filepath.Join(bundleDir, "publication.xml")
	if err := exportPublicationXML(publicationXMLPath, client, record); err != nil {
		return ExtractedBundle{}, err
	}

	recordPath := filepath.Join(bundleDir, "record.json")
	if err := writeJSONFile(recordPath, record); err != nil {
		return ExtractedBundle{}, err
	}

	draftPath := filepath.Join(bundleDir, "draft.json")
	draft := buildDraftStub(work, record, candidate, selectionRole)
	if err := writeJSONFile(draftPath, draft); err != nil {
		return ExtractedBundle{}, err
	}

	return ExtractedBundle{
		Identifier:         record.Identifier,
		SelectionRole:      selectionRole,
		SelectionReasons:   candidate.SelectionReasons,
		PreferredURL:       record.PreferredURL,
		PreferredWorkURL:   record.PreferredWorkURL,
		PublicationXMLURL:  record.PublicationXMLURL,
		Issued:             record.Issued,
		EffectiveFrom:      record.EffectiveFrom,
		EffectiveTo:        record.EffectiveTo,
		ChangeNature:       record.ChangeNature,
		ChangeCategory:     record.ChangeCategory,
		BundleDirPath:      filepath.ToSlash(filepath.Join("bundles", selection.CVDRID, record.Identifier)),
		RecordPath:         filepath.ToSlash(filepath.Join("bundles", selection.CVDRID, record.Identifier, "record.json")),
		DraftPath:          filepath.ToSlash(filepath.Join("bundles", selection.CVDRID, record.Identifier, "draft.json")),
		PublicationXMLPath: filepath.ToSlash(filepath.Join("bundles", selection.CVDRID, record.Identifier, "publication.xml")),
	}, nil
}

func buildDraftStub(work WorkTimeline, record Record, candidate Candidate, selectionRole string) DraftStub {
	municipalitySlug := slugifyMunicipality(work.Creator)
	datePart := firstNonEmpty(record.EffectiveFrom, record.Issued, time.Now().UTC().Format("2006-01-02"))
	todo := []string{
		"Extract taxable event, rate, base, exemptions, and assessment policy from publication.xml.",
		"Confirm municipality code and any region code from primary source data.",
		"Translate the ordinance into executable rules and conformance cases before promoting out of draft.",
	}
	if record.ChangeCategory == "amendment" || record.ChangeCategory == "replacement" {
		todo = append(todo, "Review prior versions in the same CVDR work timeline before encoding amendments.")
	}
	if record.ChangeCategory == "repeal" {
		todo = append(todo, "Confirm whether this version should produce executable tax rules or only a repeal marker.")
	}

	return DraftStub{
		ArtifactType: "cvdr_extraction_stub",
		Status:       "pending_manual_extraction",
		ExtractedAt:  time.Now().UTC().Format(time.RFC3339),
		Domain:       "tourist_tax",
		SourceType:   "gemeentelijke_verordening",
		Jurisdiction: DraftJurisdiction{
			CountryCode:      "NL",
			CountryName:      "Netherlands",
			MunicipalityName: work.Creator,
		},
		Selection: DraftSelection{
			Role:    selectionRole,
			Reasons: append([]string(nil), candidate.SelectionReasons...),
		},
		Source: DraftSource{
			CVDRID:            record.CVDRID,
			Identifier:        record.Identifier,
			PreferredURL:      record.PreferredURL,
			PreferredWorkURL:  record.PreferredWorkURL,
			PublicationXMLURL: record.PublicationXMLURL,
			Issued:            record.Issued,
			EffectiveFrom:     record.EffectiveFrom,
			EffectiveTo:       record.EffectiveTo,
			ChangeNature:      record.ChangeNature,
			ChangeCategory:    record.ChangeCategory,
			Title:             record.Title,
			Alternative:       record.Alternative,
		},
		SuggestedFixturePath: filepath.ToSlash(filepath.Join("core", "fixtures", "regulation", "nl", "gemeentelijke_verordening", municipalitySlug, datePart+".json")),
		TODO:                 todo,
		Work: DraftWorkDescriptor{
			CVDRID:              work.CVDRID,
			Creator:             work.Creator,
			RepresentativeTitle: work.RepresentativeTitle,
		},
	}
}

func exportPublicationXML(path string, client Client, record Record) error {
	body, err := client.FetchPublicationXML(record.PublicationXMLURL)
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func (c *Client) FetchPublicationXML(rawURL string) ([]byte, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("publication xml url is required")
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := httpClient.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("publication xml fetch failed: %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func readSelectionExportManifest(path string) (SelectionExportManifest, error) {
	var manifest SelectionExportManifest
	if err := readJSON(path, &manifest); err != nil {
		return SelectionExportManifest{}, err
	}
	return manifest, nil
}

func readSelectionWorkFiles(selectionDir string) ([]WorkSelection, error) {
	pattern := filepath.Join(selectionDir, "works", "*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	out := make([]WorkSelection, 0, len(paths))
	for _, path := range paths {
		var selection WorkSelection
		if err := readJSON(path, &selection); err != nil {
			return nil, err
		}
		out = append(out, selection)
	}
	return out, nil
}

func slugifyMunicipality(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastUnderscore = false
		case r == '\'' || r == '’':
			continue
		default:
			if !lastUnderscore {
				b.WriteRune('_')
				lastUnderscore = true
			}
		}
	}
	slug := strings.Trim(b.String(), "_")
	if slug == "" {
		return "unknown_municipality"
	}
	return slug
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
