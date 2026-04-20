package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/odeva-labs/tax-conformance-kit/engine/internal/model"
)

func TestResolveRuleSetSelectsBarcelonaCityFixture(t *testing.T) {
	repoRoot := findEngineRepoRoot(t)
	resolved, err := ResolveRuleSet(model.BookingInput{
		StayDate: "2026-06-15",
		PropertyLocation: &model.Location{
			CountryCode:  "ES",
			RegionCode:   "ES-CT",
			LocalityKind: "municipality",
			LocalityCode: "08019",
			LocalityName: "Barcelona",
		},
		AccommodationType: "hotel_5_star",
		Guests: []model.Guest{
			{Age: intPtr(34), Role: "guest"},
		},
	}, ResolveRuleSetRequest{
		FixtureRoot: filepath.Join(repoRoot, "core", "fixtures", "regulation"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resolved.Path, "/es/impuesto_estancias_turisticas/catalonia/barcelona_city/") {
		t.Fatalf("expected barcelona city fixture, got %s", resolved.Path)
	}
}

func TestResolveRuleSetSelectsRestOfCataloniaFixture(t *testing.T) {
	repoRoot := findEngineRepoRoot(t)
	resolved, err := ResolveRuleSet(model.BookingInput{
		StayDate: "2026-07-20",
		PropertyLocation: &model.Location{
			CountryCode:  "ES",
			RegionCode:   "ES-CT",
			LocalityKind: "municipality",
			LocalityCode: "17079",
			LocalityName: "Girona",
		},
		AccommodationType: "hotel_4_star",
		Guests: []model.Guest{
			{Age: intPtr(42), Role: "guest"},
		},
	}, ResolveRuleSetRequest{
		FixtureRoot: filepath.Join(repoRoot, "core", "fixtures", "regulation"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resolved.Path, "/es/impuesto_estancias_turisticas/catalonia/rest_of_catalonia/") {
		t.Fatalf("expected rest of catalonia fixture, got %s", resolved.Path)
	}
}

func TestResolveRuleSetSelectsBalearicFixture(t *testing.T) {
	repoRoot := findEngineRepoRoot(t)
	resolved, err := ResolveRuleSet(model.BookingInput{
		StayDate: "2026-11-15",
		PropertyLocation: &model.Location{
			CountryCode:  "ES",
			RegionCode:   "ES-IB",
			LocalityKind: "municipality",
			LocalityName: "Palma",
		},
		AccommodationType: "tourist_home",
		Guests: []model.Guest{
			{Age: intPtr(31), Role: "guest"},
		},
	}, ResolveRuleSetRequest{
		FixtureRoot: filepath.Join(repoRoot, "core", "fixtures", "regulation"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resolved.Path, "/es/impuesto_turismo_sostenible/balearic_islands/") {
		t.Fatalf("expected balearic fixture, got %s", resolved.Path)
	}
}

func TestResolveRuleSetRequiresCountryCode(t *testing.T) {
	repoRoot := findEngineRepoRoot(t)
	_, err := ResolveRuleSet(model.BookingInput{
		StayDate:                 "2026-06-01",
		PropertyMunicipalityCode: "0363",
		AccommodationType:        "hotel",
	}, ResolveRuleSetRequest{
		FixtureRoot: filepath.Join(repoRoot, "core", "fixtures", "regulation"),
	})
	if err == nil || !strings.Contains(err.Error(), "property_location.country_code is required") {
		t.Fatalf("expected missing country code error, got %v", err)
	}
}

func TestResolveRuleSetScopesFixtureWalkToCountryDirectory(t *testing.T) {
	clearFixtureRuleSetIndexCacheForTest()
	t.Cleanup(clearFixtureRuleSetIndexCacheForTest)

	fixtureRoot := t.TempDir()
	esPath := filepath.Join(fixtureRoot, "es", "demo", "2026-01-01.json")
	writeRuleSetFixture(t, esPath, model.RuleSet{
		ID:           "es-demo",
		Domain:       "tourist_tax",
		Jurisdiction: model.Jurisdiction{CountryCode: "ES"},
		Rules: []model.Rule{
			{
				ID:        "es-rule",
				ValidFrom: "2026-01-01",
				LocationScope: &model.Location{
					CountryCode:  "ES",
					RegionCode:   "ES-CT",
					LocalityKind: "municipality",
					LocalityCode: "08019",
				},
				Calculation: model.Calculation{Kind: "generic.per_person_per_night"},
				Source:      model.Source{SourceURL: "https://example.com/es"},
				Confidence:  "official",
			},
		},
	})

	nlPath := filepath.Join(fixtureRoot, "nl", "demo", "2026-01-01.json")
	if err := os.MkdirAll(filepath.Dir(nlPath), 0o755); err != nil {
		t.Fatalf("mkdir nl fixture dir: %v", err)
	}
	if err := os.WriteFile(nlPath, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("write malformed nl fixture: %v", err)
	}

	resolved, err := ResolveRuleSet(model.BookingInput{
		StayDate: "2026-06-15",
		PropertyLocation: &model.Location{
			CountryCode:  "ES",
			RegionCode:   "ES-CT",
			LocalityKind: "municipality",
			LocalityCode: "08019",
		},
	}, ResolveRuleSetRequest{FixtureRoot: fixtureRoot})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Path != filepath.ToSlash(esPath) {
		t.Fatalf("expected es fixture path %s, got %s", filepath.ToSlash(esPath), resolved.Path)
	}
}

func TestResolveRuleSetCachesFixtureIndex(t *testing.T) {
	clearFixtureRuleSetIndexCacheForTest()
	t.Cleanup(clearFixtureRuleSetIndexCacheForTest)

	fixtureRoot := t.TempDir()
	fixturePath := filepath.Join(fixtureRoot, "es", "demo", "2026-01-01.json")
	writeRuleSetFixture(t, fixturePath, model.RuleSet{
		ID:           "es-demo",
		Domain:       "tourist_tax",
		Jurisdiction: model.Jurisdiction{CountryCode: "ES"},
		Rules: []model.Rule{
			{
				ID:        "es-rule",
				ValidFrom: "2026-01-01",
				LocationScope: &model.Location{
					CountryCode:  "ES",
					RegionCode:   "ES-CT",
					LocalityKind: "municipality",
					LocalityCode: "08019",
				},
				Calculation: model.Calculation{Kind: "generic.per_person_per_night"},
				Source:      model.Source{SourceURL: "https://example.com/es"},
				Confidence:  "official",
			},
		},
	})

	input := model.BookingInput{
		StayDate: "2026-06-15",
		PropertyLocation: &model.Location{
			CountryCode:  "ES",
			RegionCode:   "ES-CT",
			LocalityKind: "municipality",
			LocalityCode: "08019",
		},
	}

	firstResolved, err := ResolveRuleSet(input, ResolveRuleSetRequest{FixtureRoot: fixtureRoot})
	if err != nil {
		t.Fatalf("unexpected first error: %v", err)
	}

	if err := os.WriteFile(fixturePath, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("overwrite fixture with malformed json: %v", err)
	}

	secondResolved, err := ResolveRuleSet(input, ResolveRuleSetRequest{FixtureRoot: fixtureRoot})
	if err != nil {
		t.Fatalf("unexpected second error: %v", err)
	}
	if secondResolved.Path != firstResolved.Path {
		t.Fatalf("expected cached fixture path %s, got %s", firstResolved.Path, secondResolved.Path)
	}
}

func writeRuleSetFixture(t *testing.T, path string, ruleset model.RuleSet) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir fixture dir: %v", err)
	}
	body, err := json.Marshal(ruleset)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func clearFixtureRuleSetIndexCacheForTest() {
	fixtureRuleSetIndexCache = sync.Map{}
}
