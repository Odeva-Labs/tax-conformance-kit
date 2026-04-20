package engine

import (
	"testing"

	"github.com/odeva-labs/tax-conformance-kit/engine/internal/model"
)

func intPtr(v int) *int       { return &v }
func strPtr(s string) *string { return &s }

func TestEvaluatePerPersonPerNight(t *testing.T) {
	rs := model.RuleSet{
		Jurisdiction: model.Jurisdiction{CountryCode: "NL"},
		Rules: []model.Rule{
			{
				ID:               "r1",
				MunicipalityCode: "0363",
				ValidFrom:        "2026-01-01",
				AppliesTo: model.AppliesTo{
					AccommodationTypes: []string{"camping"},
				},
				Calculation: model.Calculation{
					Kind:   "generic.per_person_per_night",
					Params: map[string]any{"amount": 2.5},
				},
			},
		},
	}

	input := model.BookingInput{
		StayDate:                 "2026-06-01",
		Nights:                   3,
		Adults:                   2,
		Children:                 1,
		PropertyMunicipalityCode: "0363",
		AccommodationType:        "camping",
		Subtotal:                 420,
	}

	got, err := Evaluate(input, rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TotalTax != 22.5 {
		t.Fatalf("expected 22.5 tax, got %v", got.TotalTax)
	}
	if len(got.MatchedRuleIDs) != 1 || got.MatchedRuleIDs[0] != "r1" {
		t.Fatalf("unexpected matched rules: %+v", got.MatchedRuleIDs)
	}
}

func TestEvaluateResidentExempt(t *testing.T) {
	rs := model.RuleSet{
		Jurisdiction: model.Jurisdiction{CountryCode: "NL"},
		Rules: []model.Rule{
			{
				ID:               "r1",
				MunicipalityCode: "0363",
				ValidFrom:        "2026-01-01",
				Exemptions: []model.Predicate{
					{Kind: "guest.resident_of_same_municipality"},
				},
				Calculation: model.Calculation{
					Kind:   "generic.fixed_amount",
					Params: map[string]any{"amount": 10.0},
				},
			},
		},
	}

	input := model.BookingInput{
		StayDate:                  "2026-06-01",
		Nights:                    2,
		Adults:                    2,
		Children:                  0,
		MainGuestMunicipalityCode: strPtr("0363"),
		PropertyMunicipalityCode:  "0363",
		AccommodationType:         "camping",
		Subtotal:                  250,
	}

	got, err := Evaluate(input, rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TotalTax != 0 {
		t.Fatalf("expected 0 tax, got %v", got.TotalTax)
	}
	if len(got.MatchedRuleIDs) != 0 {
		t.Fatalf("expected no matched rules, got %+v", got.MatchedRuleIDs)
	}
}

func TestEvaluateTieredArrangementRule(t *testing.T) {
	rs := model.RuleSet{
		Jurisdiction: model.Jurisdiction{CountryCode: "NL"},
		Rules: []model.Rule{
			{
				ID:               "tiered",
				MunicipalityCode: "0758",
				ValidFrom:        "2026-01-01",
				AppliesTo:        model.AppliesTo{AccommodationTypes: []string{"camping"}},
				Predicates: []model.Predicate{
					{Kind: "stay.accommodation_brought_by", Params: map[string]any{"value": "guest"}},
					{Kind: "stay.pricing_arrangement", Params: map[string]any{"value": "arrangement"}},
				},
				Calculation: model.Calculation{
					Kind: "nl.tiered_by_stay_duration",
					Params: map[string]any{
						"tiers": []any{
							map[string]any{"max_nights": 30.0, "amount": 20.0},
							map[string]any{"min_nights": 30.0, "max_nights": 120.0, "amount": 40.0},
						},
					},
				},
			},
		},
	}

	input := model.BookingInput{
		StayDate:                 "2026-07-01",
		Nights:                   45,
		Adults:                   2,
		PropertyMunicipalityCode: "0758",
		AccommodationType:        "camping",
		AccommodationBroughtBy:   "guest",
		PricingArrangement:       "arrangement",
	}

	got, err := Evaluate(input, rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TotalTax != 40 {
		t.Fatalf("expected 40 tax, got %v", got.TotalTax)
	}
	if len(got.MatchedRuleIDs) != 1 || got.MatchedRuleIDs[0] != "tiered" {
		t.Fatalf("unexpected matched rules: %+v", got.MatchedRuleIDs)
	}
}

func TestEvaluateSupervisedMinorGroupExempt(t *testing.T) {
	rs := model.RuleSet{
		Jurisdiction: model.Jurisdiction{CountryCode: "NL"},
		Rules: []model.Rule{
			{
				ID:               "r1",
				MunicipalityCode: "0758",
				ValidFrom:        "2026-01-01",
				AppliesTo:        model.AppliesTo{AccommodationTypes: []string{"camping"}},
				Predicates: []model.Predicate{
					{Kind: "stay.accommodation_brought_by", Params: map[string]any{"value": "guest"}},
					{Kind: "stay.pricing_arrangement", Params: map[string]any{"value": "per_night"}},
				},
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

	input := model.BookingInput{
		StayDate:                 "2026-07-01",
		Nights:                   3,
		Adults:                   2,
		Children:                 11,
		PropertyMunicipalityCode: "0758",
		AccommodationType:        "camping",
		StayPurpose:              "school_group",
		AccommodationBroughtBy:   "guest",
		PricingArrangement:       "per_night",
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

	got, err := Evaluate(input, rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TotalTax != 0 {
		t.Fatalf("expected 0 tax, got %v", got.TotalTax)
	}
	if len(got.MatchedRuleIDs) != 0 {
		t.Fatalf("expected no matched rules, got %+v", got.MatchedRuleIDs)
	}
}

func TestEvaluateDoesNotApplyAssessmentPolicyAtBookingLevel(t *testing.T) {
	rs := model.RuleSet{
		Jurisdiction: model.Jurisdiction{CountryCode: "NL"},
		AssessmentPolicy: &model.AssessmentPolicy{
			Period: "calendar_quarter",
			MinimumAssessmentAmount: &model.MinimumAmountPolicy{
				Amount:   25,
				Currency: "EUR",
			},
		},
		Rules: []model.Rule{
			{
				ID:               "r1",
				MunicipalityCode: "0363",
				ValidFrom:        "2026-01-01",
				Calculation: model.Calculation{
					Kind:   "generic.fixed_amount",
					Params: map[string]any{"amount": 10.0},
				},
			},
		},
	}

	input := model.BookingInput{
		StayDate:                 "2026-06-01",
		Nights:                   1,
		Adults:                   1,
		PropertyMunicipalityCode: "0363",
		AccommodationType:        "hotel",
	}

	got, err := Evaluate(input, rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TotalTax != 10 {
		t.Fatalf("expected booking evaluator to ignore assessment policy, got %v", got.TotalTax)
	}
}

func TestEvaluateMatchesGenericLocationScopeForCrossBorderOperator(t *testing.T) {
	rs := model.RuleSet{
		Jurisdiction: model.Jurisdiction{
			CountryCode: "ES",
			CountryName: "Spain",
			RegionCode:  "ES-CT",
			RegionName:  "Catalonia",
		},
		Rules: []model.Rule{
			{
				ID: "es-ct-barcelona",
				LocationScope: &model.Location{
					CountryCode:  "ES",
					RegionCode:   "ES-CT",
					LocalityKind: "municipality",
					LocalityCode: "08019",
					LocalityName: "Barcelona",
				},
				ValidFrom: "2026-04-01",
				AppliesTo: model.AppliesTo{AccommodationTypes: []string{"hotel"}},
				Calculation: model.Calculation{
					Kind: "generic.per_person_per_night",
					Params: map[string]any{
						"amount":                3.5,
						"max_nights":            7.0,
						"taxable_guest_age_gte": 17.0,
					},
				},
			},
		},
	}

	input := model.BookingInput{
		StayDate: "2026-06-01",
		Nights:   10,
		Adults:   2,
		Children: 1,
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
		Operator:          &model.Operator{LegalCountryCode: "NL", LegalName: "Canal Stays BV"},
		AccommodationType: "hotel",
	}

	got, err := Evaluate(input, rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TotalTax != 49 {
		t.Fatalf("expected capped adult-only tax of 49, got %v", got.TotalTax)
	}
	if len(got.MatchedRuleIDs) != 1 || got.MatchedRuleIDs[0] != "es-ct-barcelona" {
		t.Fatalf("unexpected matched rules: %+v", got.MatchedRuleIDs)
	}
}

func TestEvaluatePerPersonPerNightDiscountAfterNights(t *testing.T) {
	rs := model.RuleSet{
		Jurisdiction: model.Jurisdiction{
			CountryCode: "ES",
			RegionCode:  "ES-IB",
		},
		Rules: []model.Rule{
			{
				ID: "es-ib-hotel-4-star",
				LocationScope: &model.Location{
					CountryCode: "ES",
					RegionCode:  "ES-IB",
				},
				ValidFrom: "2025-05-17",
				AppliesTo: model.AppliesTo{AccommodationTypes: []string{"hotel_4_star"}},
				Calculation: model.Calculation{
					Kind: "generic.per_person_per_night_discount_after_nights",
					Params: map[string]any{
						"amount":                3.0,
						"discount_start_night":  9.0,
						"discount_multiplier":   0.5,
						"taxable_guest_age_gte": 16.0,
					},
				},
			},
		},
	}

	input := model.BookingInput{
		StayDate: "2026-07-10",
		Nights:   10,
		Adults:   2,
		Children: 1,
		Guests: []model.Guest{
			{Age: intPtr(40), Role: "guest"},
			{Age: intPtr(34), Role: "guest"},
			{Age: intPtr(14), Role: "guest"},
		},
		PropertyLocation: &model.Location{
			CountryCode: "ES",
			RegionCode:  "ES-IB",
		},
		AccommodationType: "hotel_4_star",
	}

	got, err := Evaluate(input, rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TotalTax != 54 {
		t.Fatalf("expected 54 tax, got %v", got.TotalTax)
	}
	if len(got.MatchedRuleIDs) != 1 || got.MatchedRuleIDs[0] != "es-ib-hotel-4-star" {
		t.Fatalf("unexpected matched rules: %+v", got.MatchedRuleIDs)
	}
}

func TestEvaluateLegacyMunicipalityRuleMatchesGenericPropertyLocation(t *testing.T) {
	rs := model.RuleSet{
		Jurisdiction: model.Jurisdiction{CountryCode: "NL"},
		Rules: []model.Rule{
			{
				ID:               "r1",
				MunicipalityCode: "0363",
				ValidFrom:        "2026-01-01",
				Calculation: model.Calculation{
					Kind:   "generic.fixed_amount",
					Params: map[string]any{"amount": 10.0},
				},
			},
		},
	}

	input := model.BookingInput{
		StayDate: "2026-06-01",
		Nights:   1,
		Adults:   1,
		PropertyLocation: &model.Location{
			CountryCode:  "NL",
			LocalityKind: "municipality",
			LocalityCode: "0363",
			LocalityName: "Amsterdam",
		},
		AccommodationType: "hotel",
	}

	got, err := Evaluate(input, rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TotalTax != 10 {
		t.Fatalf("expected 10 tax, got %v", got.TotalTax)
	}
}

func TestValidateRuleSetRejectsUnknownKind(t *testing.T) {
	rs := model.RuleSet{
		Jurisdiction: model.Jurisdiction{CountryCode: "NL"},
		Rules: []model.Rule{
			{
				ID:               "r1",
				MunicipalityCode: "0758",
				ValidFrom:        "2026-01-01",
				Calculation: model.Calculation{
					Kind:   "bogus.unknown",
					Params: map[string]any{},
				},
			},
		},
	}

	registry := model.KindRegistry{
		Calculations: map[string]model.KindEntry{
			"generic.fixed_amount": {},
		},
		Predicates: map[string]model.KindEntry{},
	}

	if err := ValidateRuleSet(rs, registry); err == nil {
		t.Fatal("expected validation error for unknown calculation kind")
	}
}

func TestValidateRuleSetAcceptsLocationScopeWithoutLegacyMunicipalityCode(t *testing.T) {
	rs := model.RuleSet{
		Jurisdiction: model.Jurisdiction{CountryCode: "ES"},
		Rules: []model.Rule{
			{
				ID: "r1",
				LocationScope: &model.Location{
					CountryCode:  "ES",
					RegionCode:   "ES-CT",
					LocalityKind: "municipality",
					LocalityCode: "08019",
				},
				ValidFrom: "2026-04-01",
				Calculation: model.Calculation{
					Kind:   "generic.fixed_amount",
					Params: map[string]any{"amount": 5.0},
				},
			},
		},
	}

	registry := model.KindRegistry{
		Calculations: map[string]model.KindEntry{
			"generic.fixed_amount": {
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
		Predicates: map[string]model.KindEntry{},
	}

	if err := ValidateRuleSet(rs, registry); err != nil {
		t.Fatalf("expected location_scope rule to validate, got %v", err)
	}
}

func TestValidateRuleSetRejectsInvalidCalculationParams(t *testing.T) {
	rs := model.RuleSet{
		Jurisdiction: model.Jurisdiction{CountryCode: "NL"},
		Rules: []model.Rule{
			{
				ID:               "r1",
				MunicipalityCode: "0363",
				ValidFrom:        "2026-01-01",
				Calculation: model.Calculation{
					Kind:   "generic.percentage_of_base",
					Params: map[string]any{"rate_pct": 7.0, "base": "bad_base"},
				},
			},
		},
	}

	registry := model.KindRegistry{
		Calculations: map[string]model.KindEntry{
			"generic.percentage_of_base": {
				ParamsSchema: map[string]any{
					"type":     "object",
					"required": []any{"rate_pct", "base"},
					"properties": map[string]any{
						"rate_pct": map[string]any{"type": "number", "minimum": 0.0},
						"base":     map[string]any{"type": "string", "enum": []any{"subtotal"}},
					},
					"additionalProperties": false,
				},
			},
		},
		Predicates: map[string]model.KindEntry{},
	}

	if err := ValidateRuleSet(rs, registry); err == nil {
		t.Fatal("expected validation error for invalid calculation params")
	}
}

func TestValidateRuleSetRejectsUnexpectedPredicateParam(t *testing.T) {
	rs := model.RuleSet{
		Jurisdiction: model.Jurisdiction{CountryCode: "NL"},
		Rules: []model.Rule{
			{
				ID:               "r1",
				MunicipalityCode: "0363",
				ValidFrom:        "2026-01-01",
				Calculation: model.Calculation{
					Kind:   "generic.fixed_amount",
					Params: map[string]any{"amount": 5.0},
				},
				Exemptions: []model.Predicate{
					{Kind: "guest.resident_of_same_municipality", Params: map[string]any{"unexpected": true}},
				},
			},
		},
	}

	registry := model.KindRegistry{
		Calculations: map[string]model.KindEntry{
			"generic.fixed_amount": {
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
			"guest.resident_of_same_municipality": {
				ParamsSchema: map[string]any{
					"type":                 "object",
					"additionalProperties": false,
				},
			},
		},
	}

	if err := ValidateRuleSet(rs, registry); err == nil {
		t.Fatal("expected validation error for unexpected predicate param")
	}
}

func TestEvaluateAssessmentAppliesMinimumAmountPolicy(t *testing.T) {
	rs := model.RuleSet{
		Jurisdiction: model.Jurisdiction{CountryCode: "NL"},
		AssessmentPolicy: &model.AssessmentPolicy{
			Period: "calendar_quarter",
			MinimumAssessmentAmount: &model.MinimumAmountPolicy{
				Amount:   25,
				Currency: "EUR",
			},
		},
		Rules: []model.Rule{
			{
				ID:               "r1",
				MunicipalityCode: "0363",
				ValidFrom:        "2026-01-01",
				Calculation: model.Calculation{
					Kind:   "generic.fixed_amount",
					Params: map[string]any{"amount": 10.0},
				},
			},
		},
	}

	got, err := EvaluateAssessment(model.AssessmentInput{
		PeriodStart: "2026-04-01",
		PeriodEnd:   "2026-06-30",
		Bookings: []model.BookingInput{
			{StayDate: "2026-04-10", PropertyMunicipalityCode: "0363", AccommodationType: "hotel"},
			{StayDate: "2026-05-15", PropertyMunicipalityCode: "0363", AccommodationType: "hotel"},
		},
	}, rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TotalBookingTax != 20 {
		t.Fatalf("expected booking tax 20, got %v", got.TotalBookingTax)
	}
	if got.TotalAssessmentTax != 0 {
		t.Fatalf("expected assessment tax 0, got %v", got.TotalAssessmentTax)
	}
}

func TestEvaluateAssessmentKeepsAmountAboveThreshold(t *testing.T) {
	rs := model.RuleSet{
		Jurisdiction: model.Jurisdiction{CountryCode: "NL"},
		AssessmentPolicy: &model.AssessmentPolicy{
			Period: "calendar_quarter",
			MinimumAssessmentAmount: &model.MinimumAmountPolicy{
				Amount:   25,
				Currency: "EUR",
			},
		},
		Rules: []model.Rule{
			{
				ID:               "r1",
				MunicipalityCode: "0363",
				ValidFrom:        "2026-01-01",
				Calculation: model.Calculation{
					Kind:   "generic.fixed_amount",
					Params: map[string]any{"amount": 10.0},
				},
			},
		},
	}

	got, err := EvaluateAssessment(model.AssessmentInput{
		PeriodStart: "2026-04-01",
		PeriodEnd:   "2026-06-30",
		Bookings: []model.BookingInput{
			{StayDate: "2026-04-10", PropertyMunicipalityCode: "0363", AccommodationType: "hotel"},
			{StayDate: "2026-05-15", PropertyMunicipalityCode: "0363", AccommodationType: "hotel"},
			{StayDate: "2026-06-20", PropertyMunicipalityCode: "0363", AccommodationType: "hotel"},
		},
	}, rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TotalBookingTax != 30 {
		t.Fatalf("expected booking tax 30, got %v", got.TotalBookingTax)
	}
	if got.TotalAssessmentTax != 30 {
		t.Fatalf("expected assessment tax 30, got %v", got.TotalAssessmentTax)
	}
}
