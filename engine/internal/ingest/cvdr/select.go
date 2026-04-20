package cvdr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type SelectionRequest struct {
	IndexDir             string
	AsOfDate             string
	MaxHistoricalPerWork int
}

type SelectionManifest struct {
	SelectedAt               string          `json:"selected_at"`
	IndexDir                 string          `json:"index_dir"`
	AsOfDate                 string          `json:"as_of_date"`
	MaxHistoricalPerWork     int             `json:"max_historical_per_work"`
	WorkCount                int             `json:"work_count"`
	CurrentCandidateCount    int             `json:"current_candidate_count"`
	HistoricalCandidateCount int             `json:"historical_candidate_count"`
	Works                    []WorkSelection `json:"works,omitempty"`
}

type WorkSelection struct {
	CVDRID               string      `json:"cvdr_id"`
	Creator              string      `json:"creator,omitempty"`
	RepresentativeTitle  string      `json:"representative_title,omitempty"`
	CurrentCandidate     Candidate   `json:"current_candidate"`
	HistoricalCandidates []Candidate `json:"historical_candidates,omitempty"`
}

type Candidate struct {
	Identifier       string   `json:"identifier"`
	PreferredURL     string   `json:"preferred_url,omitempty"`
	Issued           string   `json:"issued,omitempty"`
	EffectiveFrom    string   `json:"effective_from,omitempty"`
	EffectiveTo      string   `json:"effective_to,omitempty"`
	ChangeCategory   string   `json:"change_category,omitempty"`
	SelectionReasons []string `json:"selection_reasons"`
}

func SelectCandidates(req SelectionRequest) (SelectionManifest, error) {
	if req.IndexDir == "" {
		return SelectionManifest{}, fmt.Errorf("index directory is required")
	}
	if req.MaxHistoricalPerWork <= 0 {
		req.MaxHistoricalPerWork = 3
	}
	asOf, err := parseAsOfDate(req.AsOfDate)
	if err != nil {
		return SelectionManifest{}, err
	}

	workFiles, err := readWorkFiles(req.IndexDir)
	if err != nil {
		return SelectionManifest{}, err
	}

	selections := make([]WorkSelection, 0, len(workFiles))
	historicalCount := 0
	for _, workFile := range workFiles {
		if !isLikelyTouristTaxBaseWork(workFile) {
			continue
		}
		selection := selectWorkCandidates(workFile, asOf, req.MaxHistoricalPerWork)
		historicalCount += len(selection.HistoricalCandidates)
		selections = append(selections, selection)
	}
	sort.Slice(selections, func(i, j int) bool { return selections[i].CVDRID < selections[j].CVDRID })

	return SelectionManifest{
		SelectedAt:               time.Now().UTC().Format(time.RFC3339),
		IndexDir:                 req.IndexDir,
		AsOfDate:                 asOf.Format("2006-01-02"),
		MaxHistoricalPerWork:     req.MaxHistoricalPerWork,
		WorkCount:                len(selections),
		CurrentCandidateCount:    len(selections),
		HistoricalCandidateCount: historicalCount,
		Works:                    selections,
	}, nil
}

func ExportSelection(outputDir string, selection SelectionManifest) error {
	if outputDir == "" {
		return fmt.Errorf("output directory is required")
	}
	if err := os.MkdirAll(filepath.Join(outputDir, "works"), 0o755); err != nil {
		return err
	}

	workIndex := make([]WorkSelection, 0, len(selection.Works))
	for _, work := range selection.Works {
		workIndex = append(workIndex, WorkSelection{
			CVDRID:               work.CVDRID,
			Creator:              work.Creator,
			RepresentativeTitle:  work.RepresentativeTitle,
			CurrentCandidate:     work.CurrentCandidate,
			HistoricalCandidates: work.HistoricalCandidates,
		})
		if err := writeJSONFile(filepath.Join(outputDir, "works", work.CVDRID+".json"), work); err != nil {
			return err
		}
	}

	if err := writeJSONFile(filepath.Join(outputDir, "work_index.json"), workIndex); err != nil {
		return err
	}

	manifest := selection
	manifest.Works = nil
	return writeJSONFile(filepath.Join(outputDir, "manifest.json"), manifest)
}

func parseAsOfDate(value string) (time.Time, error) {
	if value == "" {
		return time.Now().UTC(), nil
	}
	asOf, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid as-of date")
	}
	return asOf, nil
}

func readWorkFiles(indexDir string) ([]WorkFile, error) {
	pattern := filepath.Join(indexDir, "works", "*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	workFiles := make([]WorkFile, 0, len(paths))
	for _, path := range paths {
		var workFile WorkFile
		if err := readJSON(path, &workFile); err != nil {
			return nil, err
		}
		workFiles = append(workFiles, workFile)
	}
	return workFiles, nil
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func selectWorkCandidates(workFile WorkFile, asOf time.Time, maxHistorical int) WorkSelection {
	versions := append([]Record(nil), workFile.Versions...)
	sort.Slice(versions, func(i, j int) bool {
		return compareTimelineDate(versions[i], versions[j])
	})

	currentIdx := chooseCurrentIndex(versions, asOf)
	current := versions[currentIdx]

	historical := make([]Candidate, 0, maxHistorical)
	if currentIdx > 0 {
		historical = append(historical, candidateFromRecord(versions[0], "earliest_version"))
	}

	for i := len(versions) - 1; i >= 0 && len(historical) < maxHistorical; i-- {
		if i == currentIdx {
			continue
		}
		reasons := historicalReasons(versions[i], current, i == 0)
		if len(reasons) == 0 {
			continue
		}
		candidate := candidateWithReasons(versions[i], reasons...)
		if !containsCandidate(historical, candidate.Identifier) {
			historical = append(historical, candidate)
		}
	}

	if len(historical) > maxHistorical {
		historical = historical[:maxHistorical]
	}

	return WorkSelection{
		CVDRID:               workFile.Work.CVDRID,
		Creator:              workFile.Work.Creator,
		RepresentativeTitle:  workFile.Work.RepresentativeTitle,
		CurrentCandidate:     currentCandidate(current, asOf),
		HistoricalCandidates: historical,
	}
}

func chooseCurrentIndex(versions []Record, asOf time.Time) int {
	bestIdx := len(versions) - 1
	for i, version := range versions {
		if isActiveOn(version, asOf) {
			bestIdx = i
		}
	}
	return bestIdx
}

func isActiveOn(record Record, asOf time.Time) bool {
	from := parseDateBestEffort(record.EffectiveFrom)
	to := parseDateBestEffort(record.EffectiveTo)
	if !from.IsZero() && asOf.Before(from) {
		return false
	}
	if !to.IsZero() && !asOf.Before(to) {
		return false
	}
	return true
}

func parseDateBestEffort(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02",
		time.RFC3339,
		"2006-01-02-07:00",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	if len(value) >= 10 {
		if parsed, err := time.Parse("2006-01-02", value[:10]); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func currentCandidate(record Record, asOf time.Time) Candidate {
	reasons := []string{"latest_active_version"}
	if !isActiveOn(record, asOf) {
		reasons = []string{"latest_known_version"}
	}
	return candidateWithReasons(record, reasons...)
}

func historicalReasons(record, current Record, isEarliest bool) []string {
	reasons := []string{}
	if isEarliest {
		reasons = append(reasons, "earliest_version")
	}
	if record.ChangeCategory != "" && record.ChangeCategory != "unknown" {
		reasons = append(reasons, "change_category:"+record.ChangeCategory)
	}
	if record.EffectiveFrom != "" && record.EffectiveFrom != current.EffectiveFrom {
		reasons = append(reasons, "different_effective_period")
	}
	if record.Issued != "" && record.Issued[:min(4, len(record.Issued))] != current.Issued[:min(4, len(current.Issued))] {
		reasons = append(reasons, "different_issue_year")
	}
	return dedupeStrings(reasons)
}

func candidateFromRecord(record Record, reason string) Candidate {
	return candidateWithReasons(record, reason)
}

func candidateWithReasons(record Record, reasons ...string) Candidate {
	return Candidate{
		Identifier:       record.Identifier,
		PreferredURL:     record.PreferredURL,
		Issued:           record.Issued,
		EffectiveFrom:    record.EffectiveFrom,
		EffectiveTo:      record.EffectiveTo,
		ChangeCategory:   record.ChangeCategory,
		SelectionReasons: dedupeStrings(reasons),
	}
}

func containsCandidate(candidates []Candidate, identifier string) bool {
	for _, candidate := range candidates {
		if candidate.Identifier == identifier {
			return true
		}
	}
	return false
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
