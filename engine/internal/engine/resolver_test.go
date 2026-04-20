package engine

import (
	"path/filepath"
	"strings"
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
