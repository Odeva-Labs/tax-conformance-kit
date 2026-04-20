package cvdr

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type CoverageReportRequest struct {
	SelectionDir string
	FixtureRoot  string
}

type CoverageReport struct {
	GeneratedAt                string                 `json:"generated_at"`
	SelectionDir               string                 `json:"selection_dir"`
	FixtureRoot                string                 `json:"fixture_root"`
	CurrentCandidateWorkCount  int                    `json:"current_candidate_work_count"`
	SelectionMunicipalityCount int                    `json:"selection_municipality_count"`
	FixtureMunicipalityCount   int                    `json:"fixture_municipality_count"`
	CoveredMunicipalityCount   int                    `json:"covered_municipality_count"`
	MissingMunicipalityCount   int                    `json:"missing_municipality_count"`
	ExtraFixtureCount          int                    `json:"extra_fixture_count"`
	MissingMunicipalities      []MunicipalityCoverage `json:"missing_municipalities,omitempty"`
	CoveredMunicipalities      []MunicipalityCoverage `json:"covered_municipalities,omitempty"`
	ExtraFixtures              []FixtureDirectory     `json:"extra_fixtures,omitempty"`
}

type MunicipalityCoverage struct {
	Slug           string              `json:"slug"`
	Creator        string              `json:"creator"`
	WorkCount      int                 `json:"work_count"`
	CurrentRecords []CoverageCandidate `json:"current_records"`
}

type CoverageCandidate struct {
	CVDRID              string `json:"cvdr_id"`
	Identifier          string `json:"identifier"`
	PreferredURL        string `json:"preferred_url,omitempty"`
	RepresentativeTitle string `json:"representative_title,omitempty"`
	EffectiveFrom       string `json:"effective_from,omitempty"`
	EffectiveTo         string `json:"effective_to,omitempty"`
}

type FixtureDirectory struct {
	Slug string `json:"slug"`
	Path string `json:"path"`
}

func BuildCoverageReport(req CoverageReportRequest) (CoverageReport, error) {
	if req.SelectionDir == "" {
		return CoverageReport{}, fmt.Errorf("selection directory is required")
	}
	if req.FixtureRoot == "" {
		return CoverageReport{}, fmt.Errorf("fixture root is required")
	}

	selections, err := readSelectionWorkFiles(req.SelectionDir)
	if err != nil {
		return CoverageReport{}, err
	}
	fixtures, err := readFixtureDirectories(req.FixtureRoot)
	if err != nil {
		return CoverageReport{}, err
	}

	grouped := groupSelectionsByMunicipality(selections)
	report := CoverageReport{
		GeneratedAt:                time.Now().UTC().Format(time.RFC3339),
		SelectionDir:               req.SelectionDir,
		FixtureRoot:                req.FixtureRoot,
		CurrentCandidateWorkCount:  len(selections),
		SelectionMunicipalityCount: len(grouped),
		FixtureMunicipalityCount:   len(fixtures),
		MissingMunicipalities:      []MunicipalityCoverage{},
		CoveredMunicipalities:      []MunicipalityCoverage{},
		ExtraFixtures:              []FixtureDirectory{},
	}

	coveredSlugs := map[string]struct{}{}
	for _, coverage := range grouped {
		if _, ok := fixtures[coverage.Slug]; ok {
			report.CoveredMunicipalities = append(report.CoveredMunicipalities, coverage)
			coveredSlugs[coverage.Slug] = struct{}{}
			continue
		}
		report.MissingMunicipalities = append(report.MissingMunicipalities, coverage)
	}

	for slug, path := range fixtures {
		if _, ok := coveredSlugs[slug]; ok {
			continue
		}
		report.ExtraFixtures = append(report.ExtraFixtures, FixtureDirectory{
			Slug: slug,
			Path: path,
		})
	}

	sort.Slice(report.CoveredMunicipalities, func(i, j int) bool {
		return report.CoveredMunicipalities[i].Slug < report.CoveredMunicipalities[j].Slug
	})
	sort.Slice(report.MissingMunicipalities, func(i, j int) bool {
		return report.MissingMunicipalities[i].Slug < report.MissingMunicipalities[j].Slug
	})
	sort.Slice(report.ExtraFixtures, func(i, j int) bool {
		return report.ExtraFixtures[i].Slug < report.ExtraFixtures[j].Slug
	})

	report.CoveredMunicipalityCount = len(report.CoveredMunicipalities)
	report.MissingMunicipalityCount = len(report.MissingMunicipalities)
	report.ExtraFixtureCount = len(report.ExtraFixtures)
	return report, nil
}

func groupSelectionsByMunicipality(selections []WorkSelection) []MunicipalityCoverage {
	grouped := map[string]*MunicipalityCoverage{}
	for _, selection := range selections {
		slug := slugifyMunicipality(selection.Creator)
		entry := grouped[slug]
		if entry == nil {
			entry = &MunicipalityCoverage{
				Slug:           slug,
				Creator:        selection.Creator,
				CurrentRecords: []CoverageCandidate{},
			}
			grouped[slug] = entry
		}
		entry.WorkCount++
		entry.CurrentRecords = append(entry.CurrentRecords, CoverageCandidate{
			CVDRID:              selection.CVDRID,
			Identifier:          selection.CurrentCandidate.Identifier,
			PreferredURL:        selection.CurrentCandidate.PreferredURL,
			RepresentativeTitle: selection.RepresentativeTitle,
			EffectiveFrom:       selection.CurrentCandidate.EffectiveFrom,
			EffectiveTo:         selection.CurrentCandidate.EffectiveTo,
		})
	}

	out := make([]MunicipalityCoverage, 0, len(grouped))
	for _, entry := range grouped {
		sort.Slice(entry.CurrentRecords, func(i, j int) bool {
			a := entry.CurrentRecords[i]
			b := entry.CurrentRecords[j]
			if a.EffectiveFrom == b.EffectiveFrom {
				return a.CVDRID < b.CVDRID
			}
			return a.EffectiveFrom > b.EffectiveFrom
		})
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Slug < out[j].Slug
	})
	return out
}

func readFixtureDirectories(root string) (map[string]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		out[entry.Name()] = filepath.ToSlash(filepath.Join(root, entry.Name()))
	}
	return out, nil
}
