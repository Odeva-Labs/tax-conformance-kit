package cvdr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/odeva-labs/tax-conformance-kit/engines/go/internal/model"
)

func TestGenerateDraftFixturesWritesSafeRulesets(t *testing.T) {
	extractionDir := t.TempDir()
	repoRoot := t.TempDir()
	bundleDir := filepath.Join(extractionDir, "bundles", "CVDR754024", "CVDR754024_1")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}

	draft := DraftStub{
		Jurisdiction: DraftJurisdiction{
			CountryCode:      "NL",
			CountryName:      "Netherlands",
			MunicipalityName: "Stede Broec",
		},
		Source: DraftSource{
			CVDRID:        "CVDR754024",
			PreferredURL:  "https://lokaleregelgeving.overheid.nl/CVDR754024/1",
			EffectiveFrom: "2026-01-01",
			Issued:        "2025-12-18",
		},
		SuggestedFixturePath: "core/fixtures/regulation/nl/gemeentelijke_verordening/stede_broec/2026-01-01.json",
	}
	if err := writeJSONFile(filepath.Join(bundleDir, "draft.json"), draft); err != nil {
		t.Fatalf("write draft: %v", err)
	}
	analysis := BundleAnalysis{
		Source: draft.Source,
		CandidateRules: []RuleCandidate{
			{
				ID:               "2026-01-01-rate-1",
				MunicipalityName: "Stede Broec",
				ValidFrom:        "2026-01-01",
				Calculation: model.Calculation{
					Kind:     "generic.per_person_per_night",
					Params:   map[string]any{"amount": 2.0},
					Currency: "EUR",
				},
				Exemptions: []model.Predicate{
					{Kind: "guest.resident_of_same_municipality"},
				},
				EvidenceArticleNumbers: []string{"6"},
				Notes:                  "Generated from tariff article.",
			},
		},
		AssessmentPolicyCandidate: &model.AssessmentPolicy{
			Period: "calendar_year",
		},
	}
	if err := writeJSONFile(filepath.Join(bundleDir, "analysis.json"), analysis); err != nil {
		t.Fatalf("write analysis: %v", err)
	}

	result, err := GenerateDraftFixtures(GenerateDraftFixturesRequest{
		ExtractionDir:  extractionDir,
		RepoRoot:       repoRoot,
		StrictWarnings: true,
	})
	if err != nil {
		t.Fatalf("unexpected generate error: %v", err)
	}
	if result.GeneratedCount != 1 {
		t.Fatalf("expected 1 generated fixture, got %+v", result)
	}

	target := filepath.Join(repoRoot, "core/fixtures/regulation/nl/gemeentelijke_verordening/stede_broec/2026-01-01.json")
	assertFileExists(t, target)
	var ruleset model.RuleSet
	readJSONFile(t, target, &ruleset)
	if ruleset.Lifecycle != "draft" || len(ruleset.Rules) != 1 {
		t.Fatalf("unexpected generated ruleset: %+v", ruleset)
	}
	if ruleset.Rules[0].MunicipalityCode != "TODO" {
		t.Fatalf("expected municipality code TODO, got %q", ruleset.Rules[0].MunicipalityCode)
	}
}

func TestGenerateDraftFixturesSkipsWarningsInStrictMode(t *testing.T) {
	extractionDir := t.TempDir()
	repoRoot := t.TempDir()
	bundleDir := filepath.Join(extractionDir, "bundles", "CVDR755247", "CVDR755247_1")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}

	draft := DraftStub{
		Jurisdiction: DraftJurisdiction{
			CountryCode:      "NL",
			CountryName:      "Netherlands",
			MunicipalityName: "Veldhoven",
		},
		Source: DraftSource{
			CVDRID:        "CVDR755247",
			PreferredURL:  "https://lokaleregelgeving.overheid.nl/CVDR755247/1",
			EffectiveFrom: "2026-01-01",
		},
		SuggestedFixturePath: "core/fixtures/regulation/nl/gemeentelijke_verordening/veldhoven/2026-01-01.json",
	}
	if err := writeJSONFile(filepath.Join(bundleDir, "draft.json"), draft); err != nil {
		t.Fatalf("write draft: %v", err)
	}
	analysis := BundleAnalysis{
		Source: draft.Source,
		CandidateRules: []RuleCandidate{
			{
				Calculation: model.Calculation{
					Kind:     "generic.per_person_per_night",
					Params:   map[string]any{"amount": 2.7},
					Currency: "EUR",
				},
			},
		},
		Warnings: []string{"Tariff article contains fixed standplaats amounts; draft fixture will need a separate fixed-per-pitch heuristic or kind."},
	}
	if err := writeJSONFile(filepath.Join(bundleDir, "analysis.json"), analysis); err != nil {
		t.Fatalf("write analysis: %v", err)
	}

	result, err := GenerateDraftFixtures(GenerateDraftFixturesRequest{
		ExtractionDir:  extractionDir,
		RepoRoot:       repoRoot,
		StrictWarnings: true,
	})
	if err != nil {
		t.Fatalf("unexpected generate error: %v", err)
	}
	if result.GeneratedCount != 0 || result.SkippedWarningCount != 1 {
		t.Fatalf("unexpected generate result: %+v", result)
	}
}

func TestGenerateDraftFixturesUsesMunicipalityCatalogAndReconcilesValidTo(t *testing.T) {
	extractionDir := t.TempDir()
	repoRoot := t.TempDir()

	catalogPath := filepath.Join(repoRoot, "core", "data", "nl", "municipality-codes.cbs-2026.json")
	if err := os.MkdirAll(filepath.Dir(catalogPath), 0o755); err != nil {
		t.Fatalf("mkdir catalog dir: %v", err)
	}
	if err := writeJSONFile(catalogPath, MunicipalityCatalog{
		SchemaVersion: "1",
		CountryCode:   "NL",
		Municipalities: []MunicipalityCatalogEntry{
			{Code: "0687", Name: "Middelburg (Z.)"},
		},
	}); err != nil {
		t.Fatalf("write catalog: %v", err)
	}

	for _, tc := range []struct {
		cvdrID        string
		effectiveFrom string
		amount        float64
	}{
		{cvdrID: "CVDR100", effectiveFrom: "2025-01-01", amount: 2.0},
		{cvdrID: "CVDR200", effectiveFrom: "2026-01-01", amount: 3.0},
	} {
		bundleDir := filepath.Join(extractionDir, "bundles", tc.cvdrID, tc.cvdrID+"_1")
		if err := os.MkdirAll(bundleDir, 0o755); err != nil {
			t.Fatalf("mkdir bundle: %v", err)
		}
		draft := DraftStub{
			Jurisdiction: DraftJurisdiction{
				CountryCode:      "NL",
				CountryName:      "Netherlands",
				MunicipalityName: "Middelburg",
			},
			Source: DraftSource{
				CVDRID:        tc.cvdrID,
				PreferredURL:  "https://example.invalid/" + tc.cvdrID,
				EffectiveFrom: tc.effectiveFrom,
			},
			SuggestedFixturePath: "core/fixtures/regulation/nl/gemeentelijke_verordening/middelburg/" + tc.effectiveFrom + ".json",
		}
		if err := writeJSONFile(filepath.Join(bundleDir, "draft.json"), draft); err != nil {
			t.Fatalf("write draft: %v", err)
		}
		if err := writeJSONFile(filepath.Join(bundleDir, "analysis.json"), BundleAnalysis{
			Source: draft.Source,
			CandidateRules: []RuleCandidate{
				{
					ID:               tc.effectiveFrom + "-rate",
					MunicipalityName: "Middelburg",
					ValidFrom:        tc.effectiveFrom,
					Calculation: model.Calculation{
						Kind:     "generic.per_person_per_night",
						Params:   map[string]any{"amount": tc.amount},
						Currency: "EUR",
					},
				},
			},
		}); err != nil {
			t.Fatalf("write analysis: %v", err)
		}
	}

	result, err := GenerateDraftFixtures(GenerateDraftFixturesRequest{
		ExtractionDir:           extractionDir,
		RepoRoot:                repoRoot,
		StrictWarnings:          true,
		MunicipalityCatalogPath: catalogPath,
	})
	if err != nil {
		t.Fatalf("generate fixtures: %v", err)
	}
	if result.GeneratedCount != 2 {
		t.Fatalf("expected 2 generated fixtures, got %+v", result)
	}

	var older model.RuleSet
	readJSONFile(t, filepath.Join(repoRoot, "core/fixtures/regulation/nl/gemeentelijke_verordening/middelburg/2025-01-01.json"), &older)
	if older.Rules[0].MunicipalityCode != "0687" {
		t.Fatalf("expected mapped municipality code, got %q", older.Rules[0].MunicipalityCode)
	}
	if older.Rules[0].ValidTo == nil || *older.Rules[0].ValidTo != "2025-12-31" {
		t.Fatalf("expected older rule valid_to 2025-12-31, got %+v", older.Rules[0].ValidTo)
	}

	var newer model.RuleSet
	readJSONFile(t, filepath.Join(repoRoot, "core/fixtures/regulation/nl/gemeentelijke_verordening/middelburg/2026-01-01.json"), &newer)
	if newer.Rules[0].ValidTo != nil {
		t.Fatalf("expected newest rule valid_to to remain nil, got %+v", newer.Rules[0].ValidTo)
	}
	if strings.Contains(newer.Notes, "municipality_code is TODO") {
		t.Fatalf("expected catalog-backed ruleset notes without TODO, got %q", newer.Notes)
	}
}
