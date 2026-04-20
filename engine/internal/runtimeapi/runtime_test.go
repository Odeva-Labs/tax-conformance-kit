package runtimeapi

import (
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
