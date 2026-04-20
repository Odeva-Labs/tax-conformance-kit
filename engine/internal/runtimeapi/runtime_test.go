package runtimeapi

import (
	"path/filepath"
	"testing"

	"github.com/odeva-labs/tax-conformance-kit/engine/internal/model"
)

func intPtr(v int) *int { return &v }

func testRegistry() model.KindRegistry {
	return model.KindRegistry{
		Calculations: map[string]model.KindEntry{
			"generic.per_person_per_night": {
				ParamsSchema: map[string]any{
					"type":     "object",
					"required": []any{"amount"},
					"properties": map[string]any{
						"amount": map[string]any{"type": "number", "minimum": 0.0},
					},
					"additionalProperties": false,
				},
			},
		},
		Predicates: map[string]model.KindEntry{
			"stay.supervised_minor_group": {
				ParamsSchema: map[string]any{
					"type":     "object",
					"required": []any{"min_minors", "min_supervisors", "supervisor_min_age", "allowed_purposes"},
					"properties": map[string]any{
						"min_minors":         map[string]any{"type": "number", "minimum": 0.0},
						"min_supervisors":    map[string]any{"type": "number", "minimum": 0.0},
						"supervisor_min_age": map[string]any{"type": "number", "minimum": 0.0},
						"allowed_purposes": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
						},
					},
					"additionalProperties": false,
				},
			},
		},
	}
}

func testRuleSet() model.RuleSet {
	return model.RuleSet{
		Jurisdiction: model.Jurisdiction{CountryCode: "NL"},
		Rules: []model.Rule{
			{
				ID:               "r1",
				MunicipalityCode: "0758",
				ValidFrom:        "2026-01-01",
				AppliesTo:        model.AppliesTo{AccommodationTypes: []string{"camping"}},
				Exemptions: []model.Predicate{
					{
						Kind: "stay.supervised_minor_group",
						Params: map[string]any{
							"min_minors":         11.0,
							"min_supervisors":    1.0,
							"supervisor_min_age": 18.0,
							"allowed_purposes":   []any{"school_group"},
						},
					},
				},
				Calculation: model.Calculation{
					Kind:   "generic.per_person_per_night",
					Params: map[string]any{"amount": 0.5},
				},
			},
		},
	}
}

func testBookingInput() model.BookingInput {
	return model.BookingInput{
		StayDate:                 "2026-07-01",
		Nights:                   3,
		Adults:                   2,
		Children:                 11,
		PropertyMunicipalityCode: "0758",
		AccommodationType:        "camping",
		StayPurpose:              "school_group",
		Guests: []model.Guest{
			{Age: intPtr(14), Role: "guest"},
			{Age: intPtr(14), Role: "guest"},
			{Age: intPtr(14), Role: "guest"},
			{Age: intPtr(14), Role: "guest"},
			{Age: intPtr(14), Role: "guest"},
			{Age: intPtr(14), Role: "guest"},
			{Age: intPtr(14), Role: "guest"},
			{Age: intPtr(14), Role: "guest"},
			{Age: intPtr(14), Role: "guest"},
			{Age: intPtr(14), Role: "guest"},
			{Age: intPtr(14), Role: "guest"},
			{Age: intPtr(30), Role: "supervisor"},
			{Age: intPtr(35), Role: "supervisor"},
		},
	}
}

func TestValidateUsesInlineRegistry(t *testing.T) {
	response := Validate(model.RuntimeValidateRequest{
		RuleSet:      testRuleSet(),
		KindRegistry: &model.KindRegistry{Calculations: testRegistry().Calculations, Predicates: testRegistry().Predicates},
	}, model.KindRegistry{})

	if !response.OK {
		t.Fatalf("expected validation to pass, got %+v", response)
	}
	if response.APIVersion != model.RuntimeAPIVersion {
		t.Fatalf("expected api version %s, got %s", model.RuntimeAPIVersion, response.APIVersion)
	}
}

func TestEvaluateReturnsStructuredResult(t *testing.T) {
	response := Evaluate(model.RuntimeEvaluateRequest{
		RuleSet:      testRuleSet(),
		BookingInput: testBookingInput(),
	}, testRegistry())

	if !response.OK {
		t.Fatalf("expected evaluation to pass, got %+v", response)
	}
	if response.Result == nil {
		t.Fatal("expected evaluation result")
	}
	if response.Result.TotalTax != 0 {
		t.Fatalf("expected exempt stay to produce zero tax, got %v", response.Result.TotalTax)
	}
}

func TestResolveEvaluateReturnsResolvedRuleSet(t *testing.T) {
	repoRoot := findRepoRoot(t)
	registry := readKindRegistry(t, filepath.Join(repoRoot, "core", "schemas", "kind-registry.v1.json"))

	response := ResolveEvaluate(model.RuntimeResolveEvaluateRequest{
		FixtureRoot: filepath.Join(repoRoot, "core", "fixtures", "regulation"),
		BookingInput: model.BookingInput{
			StayDate: "2026-06-15",
			Nights:   10,
			Guests: []model.Guest{
				{Age: intPtr(34), Role: "guest"},
				{Age: intPtr(38), Role: "guest"},
				{Age: intPtr(15), Role: "guest"},
			},
			PropertyLocation: &model.Location{
				CountryCode:  "ES",
				RegionCode:   "ES-CT",
				LocalityKind: "municipality",
				LocalityCode: "08019",
				LocalityName: "Barcelona",
			},
			AccommodationType: "hotel_5_star",
		},
	}, registry)

	if !response.OK {
		t.Fatalf("expected resolution to pass, got %+v", response)
	}
	if response.Result == nil || response.Result.TotalTax != 168 {
		t.Fatalf("expected resolved evaluation tax 168, got %+v", response.Result)
	}
	if response.ResolvedRuleSetID != "es-catalonia-barcelona-city-2026-04-01" {
		t.Fatalf("unexpected resolved ruleset id %q", response.ResolvedRuleSetID)
	}
}

func TestResolveEvaluateAssessmentGroupsByResolvedRuleSet(t *testing.T) {
	repoRoot := findRepoRoot(t)
	registry := readKindRegistry(t, filepath.Join(repoRoot, "core", "schemas", "kind-registry.v1.json"))

	response := ResolveEvaluateAssessment(model.RuntimeResolveEvaluateAssessmentRequest{
		FixtureRoot: filepath.Join(repoRoot, "core", "fixtures", "regulation"),
		AssessmentInput: model.AssessmentInput{
			PeriodStart: "2026-07-01",
			PeriodEnd:   "2026-09-30",
			Bookings: []model.BookingInput{
				{
					StayDate: "2026-07-10",
					Nights:   10,
					Guests: []model.Guest{
						{Age: intPtr(34), Role: "guest"},
						{Age: intPtr(38), Role: "guest"},
						{Age: intPtr(15), Role: "guest"},
					},
					PropertyLocation: &model.Location{
						CountryCode:  "ES",
						RegionCode:   "ES-CT",
						LocalityKind: "municipality",
						LocalityCode: "08019",
						LocalityName: "Barcelona",
					},
					AccommodationType: "hotel_5_star",
				},
				{
					StayDate: "2026-07-20",
					Nights:   10,
					Guests: []model.Guest{
						{Age: intPtr(40), Role: "guest"},
						{Age: intPtr(34), Role: "guest"},
						{Age: intPtr(14), Role: "guest"},
					},
					PropertyLocation: &model.Location{
						CountryCode:  "ES",
						RegionCode:   "ES-IB",
						LocalityKind: "municipality",
						LocalityName: "Palma",
					},
					AccommodationType: "hotel_4_star",
				},
			},
		},
	}, registry)

	if !response.OK {
		t.Fatalf("expected grouped resolution to pass, got %+v", response)
	}
	if response.GroupCount != 2 {
		t.Fatalf("expected 2 groups, got %d", response.GroupCount)
	}
	if response.TotalBookingTax != 222 || response.TotalAssessmentTax != 222 {
		t.Fatalf("expected aggregate taxes 222/222, got %v/%v", response.TotalBookingTax, response.TotalAssessmentTax)
	}
	if len(response.ResolvedAssessments) != 2 {
		t.Fatalf("expected 2 resolved assessments, got %+v", response.ResolvedAssessments)
	}
	if response.ResolvedAssessments[0].ResolvedRuleSetID != "es-catalonia-barcelona-city-2026-04-01" {
		t.Fatalf("unexpected first resolved ruleset %q", response.ResolvedAssessments[0].ResolvedRuleSetID)
	}
	if response.ResolvedAssessments[0].Result.TotalAssessmentTax != 168 {
		t.Fatalf("expected first assessment tax 168, got %v", response.ResolvedAssessments[0].Result.TotalAssessmentTax)
	}
	if response.ResolvedAssessments[1].ResolvedRuleSetID != "es-balearic-islands-2025-05-17" {
		t.Fatalf("unexpected second resolved ruleset %q", response.ResolvedAssessments[1].ResolvedRuleSetID)
	}
	if response.ResolvedAssessments[1].Result.TotalAssessmentTax != 54 {
		t.Fatalf("expected second assessment tax 54, got %v", response.ResolvedAssessments[1].Result.TotalAssessmentTax)
	}
}

func TestEvaluateAssessmentReturnsStructuredError(t *testing.T) {
	response := EvaluateAssessment(model.RuntimeEvaluateAssessmentRequest{
		RuleSet: testRuleSet(),
		AssessmentInput: model.AssessmentInput{
			PeriodStart: "2026-04-01",
			PeriodEnd:   "2026-06-30",
			Bookings: []model.BookingInput{
				testBookingInput(),
			},
		},
	}, testRegistry())

	if response.OK {
		t.Fatalf("expected evaluation to fail, got %+v", response)
	}
	if response.Error == nil || response.Error.Message == "" {
		t.Fatalf("expected structured error, got %+v", response)
	}
}

func TestValidateRejectsMissingRegistry(t *testing.T) {
	response := Validate(model.RuntimeValidateRequest{
		RuleSet: testRuleSet(),
	}, model.KindRegistry{})

	if response.OK {
		t.Fatalf("expected missing registry failure, got %+v", response)
	}
	if response.Error == nil || response.Error.Message != errNoRegistry.Error() {
		t.Fatalf("expected missing registry error, got %+v", response)
	}
}
