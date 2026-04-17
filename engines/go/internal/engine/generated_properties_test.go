package engine

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/odeva-labs/tax-conformance-kit/engines/go/internal/model"
)

func TestGeneratedRuleDateBoundaries(t *testing.T) {
	repoRoot := findEngineRepoRoot(t)
	for _, fixture := range collectRuleSetFixtures(t, repoRoot) {
		ruleset := readEngineRuleSetFixture(t, fixture)
		for _, rule := range ruleset.Rules {
			rule := rule
			testName := fmt.Sprintf("%s/%s", strings.TrimPrefix(filepath.ToSlash(fixture), filepath.ToSlash(repoRoot)+"/"), rule.ID)
			t.Run(testName, func(t *testing.T) {
				if hasPredicateKind(rule, "stay.seasonal_window") {
					t.Skip("seasonal window predicates need a separate boundary generator")
				}
				if _, ok := GetCalculation(rule.Calculation.Kind); !ok {
					t.Skipf("no calculation handler for %s", rule.Calculation.Kind)
				}

				single := model.RuleSet{
					ID:               ruleset.ID,
					Domain:           ruleset.Domain,
					Lifecycle:        ruleset.Lifecycle,
					Jurisdiction:     ruleset.Jurisdiction,
					AssessmentPolicy: ruleset.AssessmentPolicy,
					Rules:            []model.Rule{rule},
				}

				onStartInput, err := buildApplicableBookingInput(rule, rule.ValidFrom)
				if err != nil {
					t.Fatalf("build applicable booking input: %v", err)
				}
				onStart, err := Evaluate(onStartInput, single)
				if err != nil {
					t.Fatalf("evaluate on valid_from: %v", err)
				}
				if !containsRule(onStart.MatchedRuleIDs, rule.ID) {
					t.Fatalf("expected rule to match on valid_from, got %+v", onStart.MatchedRuleIDs)
				}

				beforeStartDate := mustShiftDate(t, rule.ValidFrom, -1)
				beforeStartInput, err := buildApplicableBookingInput(rule, beforeStartDate)
				if err != nil {
					t.Fatalf("build boundary booking input before valid_from: %v", err)
				}
				beforeStart, err := Evaluate(beforeStartInput, single)
				if err != nil {
					t.Fatalf("evaluate before valid_from: %v", err)
				}
				if containsRule(beforeStart.MatchedRuleIDs, rule.ID) || beforeStart.TotalTax != 0 {
					t.Fatalf("expected no match before valid_from, got tax=%v rules=%+v", beforeStart.TotalTax, beforeStart.MatchedRuleIDs)
				}

				if rule.ValidTo == nil {
					return
				}

				onEndInput, err := buildApplicableBookingInput(rule, *rule.ValidTo)
				if err != nil {
					t.Fatalf("build applicable booking input on valid_to: %v", err)
				}
				onEnd, err := Evaluate(onEndInput, single)
				if err != nil {
					t.Fatalf("evaluate on valid_to: %v", err)
				}
				if !containsRule(onEnd.MatchedRuleIDs, rule.ID) {
					t.Fatalf("expected rule to match on valid_to, got %+v", onEnd.MatchedRuleIDs)
				}

				afterEndDate := mustShiftDate(t, *rule.ValidTo, 1)
				afterEndInput, err := buildApplicableBookingInput(rule, afterEndDate)
				if err != nil {
					t.Fatalf("build boundary booking input after valid_to: %v", err)
				}
				afterEnd, err := Evaluate(afterEndInput, single)
				if err != nil {
					t.Fatalf("evaluate after valid_to: %v", err)
				}
				if containsRule(afterEnd.MatchedRuleIDs, rule.ID) || afterEnd.TotalTax != 0 {
					t.Fatalf("expected no match after valid_to, got tax=%v rules=%+v", afterEnd.TotalTax, afterEnd.MatchedRuleIDs)
				}
			})
		}
	}
}

func TestGeneratedPercentageOfBaseProperties(t *testing.T) {
	repoRoot := findEngineRepoRoot(t)
	for _, fixture := range collectRuleSetFixtures(t, repoRoot) {
		ruleset := readEngineRuleSetFixture(t, fixture)
		for _, rule := range ruleset.Rules {
			rule := rule
			if rule.Calculation.Kind != "generic.percentage_of_base" {
				continue
			}

			testName := fmt.Sprintf("%s/%s", strings.TrimPrefix(filepath.ToSlash(fixture), filepath.ToSlash(repoRoot)+"/"), rule.ID)
			t.Run(testName, func(t *testing.T) {
				single := model.RuleSet{
					ID:               ruleset.ID,
					Domain:           ruleset.Domain,
					Lifecycle:        ruleset.Lifecycle,
					Jurisdiction:     ruleset.Jurisdiction,
					AssessmentPolicy: ruleset.AssessmentPolicy,
					Rules:            []model.Rule{rule},
				}

				zeroInput, err := buildApplicableBookingInput(rule, rule.ValidFrom)
				if err != nil {
					t.Fatalf("build zero subtotal input: %v", err)
				}
				zeroInput.Subtotal = 0
				zeroResult, err := Evaluate(zeroInput, single)
				if err != nil {
					t.Fatalf("evaluate zero subtotal input: %v", err)
				}
				if zeroResult.TotalTax != 0 {
					t.Fatalf("expected zero subtotal to yield zero tax, got %v", zeroResult.TotalTax)
				}

				lowInput, err := buildApplicableBookingInput(rule, rule.ValidFrom)
				if err != nil {
					t.Fatalf("build low subtotal input: %v", err)
				}
				lowInput.Subtotal = 100
				lowResult, err := Evaluate(lowInput, single)
				if err != nil {
					t.Fatalf("evaluate low subtotal input: %v", err)
				}
				if !containsRule(lowResult.MatchedRuleIDs, rule.ID) {
					t.Fatalf("expected rule to match low subtotal input, got %+v", lowResult.MatchedRuleIDs)
				}

				highInput := lowInput
				highInput.Subtotal = 200
				highResult, err := Evaluate(highInput, single)
				if err != nil {
					t.Fatalf("evaluate high subtotal input: %v", err)
				}
				if !containsRule(highResult.MatchedRuleIDs, rule.ID) {
					t.Fatalf("expected rule to match high subtotal input, got %+v", highResult.MatchedRuleIDs)
				}
				if highResult.TotalTax < lowResult.TotalTax {
					t.Fatalf("expected percentage tax to be monotonic with subtotal, got low=%v high=%v", lowResult.TotalTax, highResult.TotalTax)
				}
			})
		}
	}
}

func TestGeneratedTierBoundaries(t *testing.T) {
	repoRoot := findEngineRepoRoot(t)
	for _, fixture := range collectRuleSetFixtures(t, repoRoot) {
		ruleset := readEngineRuleSetFixture(t, fixture)
		for _, rule := range ruleset.Rules {
			rule := rule
			if rule.Calculation.Kind != "nl.tiered_by_stay_duration" {
				continue
			}

			testName := fmt.Sprintf("%s/%s", strings.TrimPrefix(filepath.ToSlash(fixture), filepath.ToSlash(repoRoot)+"/"), rule.ID)
			t.Run(testName, func(t *testing.T) {
				single := model.RuleSet{
					ID:               ruleset.ID,
					Domain:           ruleset.Domain,
					Lifecycle:        ruleset.Lifecycle,
					Jurisdiction:     ruleset.Jurisdiction,
					AssessmentPolicy: ruleset.AssessmentPolicy,
					Rules:            []model.Rule{rule},
				}

				rawTiers, ok := rule.Calculation.Params["tiers"].([]any)
				if !ok {
					t.Fatalf("tiers must be an array")
				}

				for index, rawTier := range rawTiers {
					tier, ok := rawTier.(map[string]any)
					if !ok {
						t.Fatalf("tier must be an object")
					}
					amount, ok := tier["amount"].(float64)
					if !ok {
						t.Fatalf("tier amount must be a number")
					}

					sampleNights := tierRepresentativeNights(t, rawTiers, index, tier)
					for _, nights := range sampleNights {
						caseName := fmt.Sprintf("%d_nights", nights)
						t.Run(caseName, func(t *testing.T) {
							input, err := buildApplicableBookingInput(rule, rule.ValidFrom)
							if err != nil {
								t.Fatalf("build applicable booking input: %v", err)
							}
							input.Nights = nights

							result, err := Evaluate(input, single)
							if err != nil {
								t.Fatalf("evaluate tier boundary input: %v", err)
							}
							if !containsRule(result.MatchedRuleIDs, rule.ID) {
								t.Fatalf("expected rule to match for %d nights, got %+v", nights, result.MatchedRuleIDs)
							}
							if result.TotalTax != amount {
								t.Fatalf("expected %v tax for %d nights, got %v", amount, nights, result.TotalTax)
							}
						})
					}
				}
			})
		}
	}
}

func findEngineRepoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	dir := wd
	for {
		candidate := filepath.Join(dir, "core", "schemas", "kind-registry.v1.json")
		if _, err := os.Stat(candidate); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repo root")
		}
		dir = parent
	}
}

func collectRuleSetFixtures(t *testing.T, repoRoot string) []string {
	t.Helper()

	paths := make([]string, 0)
	err := filepath.WalkDir(filepath.Join(repoRoot, "core", "fixtures"), func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		if strings.Contains(filepath.ToSlash(path), "/conformance/") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk ruleset fixtures: %v", err)
	}
	return paths
}

func readEngineRuleSetFixture(t *testing.T, path string) model.RuleSet {
	t.Helper()

	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ruleset fixture %s: %v", path, err)
	}

	var ruleset model.RuleSet
	if err := json.Unmarshal(bytes, &ruleset); err != nil {
		t.Fatalf("unmarshal ruleset fixture %s: %v", path, err)
	}
	return ruleset
}

func buildApplicableBookingInput(rule model.Rule, stayDate string) (model.BookingInput, error) {
	differentMunicipality := "9999"
	input := model.BookingInput{
		StayDate:                  stayDate,
		Nights:                    1,
		Adults:                    1,
		Children:                  0,
		MainGuestMunicipalityCode: &differentMunicipality,
		PropertyMunicipalityCode:  rule.MunicipalityCode,
		AccommodationType:         "hotel",
		Subtotal:                  100,
		AlreadySubjectTo:          []string{},
	}

	if len(rule.AppliesTo.AccommodationTypes) > 0 {
		input.AccommodationType = rule.AppliesTo.AccommodationTypes[0]
	}

	for _, predicate := range rule.Predicates {
		if err := satisfyPredicate(&input, predicate, true); err != nil {
			return model.BookingInput{}, fmt.Errorf("predicate %s: %w", predicate.Kind, err)
		}
	}
	for _, exemption := range rule.Exemptions {
		if err := satisfyPredicate(&input, exemption, false); err != nil {
			return model.BookingInput{}, fmt.Errorf("exemption %s: %w", exemption.Kind, err)
		}
	}

	return input, nil
}

func satisfyPredicate(input *model.BookingInput, predicate model.Predicate, target bool) error {
	params := predicate.Params
	if params == nil {
		params = map[string]any{}
	}

	switch predicate.Kind {
	case "guest.resident_of_same_municipality":
		if target {
			code := input.PropertyMunicipalityCode
			input.MainGuestMunicipalityCode = &code
		} else {
			code := "9999"
			if strings.EqualFold(code, input.PropertyMunicipalityCode) {
				code = "9998"
			}
			input.MainGuestMunicipalityCode = &code
		}
	case "guest.age_below":
		maxAge, err := getInt(params, "max_age")
		if err != nil {
			return err
		}
		count := 1
		if _, ok := params["count"]; ok {
			count, err = getInt(params, "count")
			if err != nil {
				return err
			}
		}
		age := maxAge - 1
		if !target {
			age = maxAge
		}
		if age < 0 {
			age = 0
		}

		guests := make([]model.Guest, 0, max(1, count))
		for i := 0; i < count; i++ {
			a := age
			guests = append(guests, model.Guest{Age: &a, Role: "guest"})
		}
		if !target && len(guests) == 0 {
			a := maxAge
			guests = append(guests, model.Guest{Age: &a, Role: "guest"})
		}
		input.Guests = guests
		input.Adults = len(guests)
		input.Children = 0
	case "stay.purpose_in":
		values, err := getStringSlice(params, "values")
		if err != nil {
			return err
		}
		if target {
			input.StayPurpose = values[0]
		} else {
			input.StayPurpose = "business"
			for containsTestString(values, input.StayPurpose) {
				input.StayPurpose += "_other"
			}
		}
	case "stay.wtza_care_institution":
		input.WTZACareInstitution = target
	case "stay.coa_asylum_housing":
		input.COAAsylumHousing = target
	case "stay.accommodation_brought_by":
		value, err := getString(params, "value")
		if err != nil {
			return err
		}
		if target {
			input.AccommodationBroughtBy = value
		} else if value == "guest" {
			input.AccommodationBroughtBy = "operator"
		} else {
			input.AccommodationBroughtBy = "guest"
		}
	case "stay.pricing_arrangement":
		value, err := getString(params, "value")
		if err != nil {
			return err
		}
		if target {
			input.PricingArrangement = value
		} else if value == "arrangement" {
			input.PricingArrangement = "per_night"
		} else {
			input.PricingArrangement = "arrangement"
		}
	case "stay.supervised_minor_group":
		minMinors, err := getInt(params, "min_minors")
		if err != nil {
			return err
		}
		minSupervisors, err := getInt(params, "min_supervisors")
		if err != nil {
			return err
		}
		supervisorMinAge, err := getInt(params, "supervisor_min_age")
		if err != nil {
			return err
		}
		if _, ok := params["allowed_purposes"]; ok {
			values, err := getStringSlice(params, "allowed_purposes")
			if err != nil {
				return err
			}
			if target {
				input.StayPurpose = values[0]
			} else {
				input.StayPurpose = "business"
			}
		}

		if target {
			guests := make([]model.Guest, 0, minMinors+minSupervisors)
			for i := 0; i < minMinors; i++ {
				age := 14
				guests = append(guests, model.Guest{Age: &age, Role: "guest"})
			}
			for i := 0; i < minSupervisors; i++ {
				age := supervisorMinAge
				guests = append(guests, model.Guest{Age: &age, Role: "supervisor"})
			}
			input.Guests = guests
			input.Adults = minSupervisors
			input.Children = minMinors
		} else {
			age := supervisorMinAge
			input.Guests = []model.Guest{{Age: &age, Role: "supervisor"}}
			input.Adults = 1
			input.Children = 0
		}
	case "cross_tax.already_subject_to":
		tax, err := getString(params, "tax")
		if err != nil {
			return err
		}
		if target {
			input.AlreadySubjectTo = []string{tax}
		} else {
			input.AlreadySubjectTo = []string{}
		}
	case "stay.seasonal_window":
		return fmt.Errorf("seasonal window predicates are not supported in generated properties")
	default:
		return fmt.Errorf("unsupported predicate kind %s", predicate.Kind)
	}

	return nil
}

func tierRepresentativeNights(t *testing.T, rawTiers []any, index int, tier map[string]any) []int {
	t.Helper()

	result := make([]int, 0, 2)
	minNights := 0
	if rawMin, ok := tier["min_nights"]; ok {
		min, ok := rawMin.(float64)
		if !ok {
			t.Fatalf("tier min_nights must be a number")
		}
		minNights = int(min)
	}
	maxNights := 0
	hasMax := false
	if rawMax, ok := tier["max_nights"]; ok {
		max, ok := rawMax.(float64)
		if !ok {
			t.Fatalf("tier max_nights must be a number")
		}
		maxNights = int(max)
		hasMax = true
	}

	if minNights <= 1 {
		result = append(result, 1)
	} else {
		result = append(result, minNights)
	}

	if hasMax && maxNights > minNights {
		insideUpper := maxNights - 1
		if insideUpper >= 1 && insideUpper != result[0] {
			result = append(result, insideUpper)
		}
	}

	if index > 0 && len(result) == 1 {
		prevTier, ok := rawTiers[index-1].(map[string]any)
		if ok {
			if rawPrevMax, exists := prevTier["max_nights"]; exists {
				if prevMax, ok := rawPrevMax.(float64); ok {
					boundary := int(prevMax)
					if boundary >= 1 && boundary != result[0] {
						result = append(result, boundary)
					}
				}
			}
		}
	}

	return result
}

func containsRule(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func containsTestString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func mustShiftDate(t *testing.T, date string, days int) string {
	t.Helper()

	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		t.Fatalf("parse date %s: %v", date, err)
	}
	return parsed.AddDate(0, 0, days).Format("2006-01-02")
}

func hasPredicateKind(rule model.Rule, kind string) bool {
	for _, predicate := range rule.Predicates {
		if predicate.Kind == kind {
			return true
		}
	}
	for _, exemption := range rule.Exemptions {
		if exemption.Kind == kind {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
