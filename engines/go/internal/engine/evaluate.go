package engine

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/odeva-labs/tax-conformance-kit/engines/go/internal/model"
)

func Evaluate(input model.BookingInput, rs model.RuleSet) (model.EvaluationResult, error) {
	if input.Nights < 0 {
		return model.EvaluationResult{}, errors.New("nights must be >= 0")
	}

	totalTax := 0.0
	matched := make([]string, 0)
	stayDate, err := time.Parse("2006-01-02", input.StayDate)
	if err != nil {
		return model.EvaluationResult{}, errors.New("invalid stay_date")
	}

	for _, rule := range rs.Rules {
		applies, evalErr := ruleApplies(rule, input, stayDate)
		if evalErr != nil {
			return model.EvaluationResult{}, evalErr
		}
		if !applies {
			continue
		}

		handler, ok := GetCalculation(rule.Calculation.Kind)
		if !ok {
			return model.EvaluationResult{}, fmt.Errorf("no handler for calculation kind %s", rule.Calculation.Kind)
		}

		tax, evalErr := handler(rule.Calculation.Params, input)
		if evalErr != nil {
			return model.EvaluationResult{}, fmt.Errorf("rule %s: %w", rule.ID, evalErr)
		}
		totalTax += tax
		matched = append(matched, rule.ID)
	}

	return model.EvaluationResult{
		TotalTax:       round2(totalTax),
		MatchedRuleIDs: matched,
	}, nil
}

func ruleApplies(rule model.Rule, input model.BookingInput, stayDate time.Time) (bool, error) {
	if rule.MunicipalityCode != input.PropertyMunicipalityCode {
		return false, nil
	}

	if len(rule.AppliesTo.AccommodationTypes) > 0 && !slices.Contains(rule.AppliesTo.AccommodationTypes, input.AccommodationType) {
		return false, nil
	}

	if !dateInRange(stayDate, rule.ValidFrom, rule.ValidTo) {
		return false, nil
	}

	for _, predicate := range rule.Predicates {
		matches, err := evalPredicate(predicate, input)
		if err != nil {
			return false, fmt.Errorf("rule %s predicate %s: %w", rule.ID, predicate.Kind, err)
		}
		if !matches {
			return false, nil
		}
	}

	for _, exemption := range rule.Exemptions {
		matches, err := evalPredicate(exemption, input)
		if err != nil {
			return false, fmt.Errorf("rule %s exemption %s: %w", rule.ID, exemption.Kind, err)
		}
		if matches {
			return false, nil
		}
	}

	return true, nil
}

func evalPredicate(predicate model.Predicate, input model.BookingInput) (bool, error) {
	handler, ok := GetPredicate(predicate.Kind)
	if !ok {
		return false, fmt.Errorf("no handler for predicate kind %s", predicate.Kind)
	}
	return handler(predicate.Params, input)
}

func dateInRange(stayDate time.Time, validFrom string, validTo *string) bool {
	from, err := time.Parse("2006-01-02", validFrom)
	if err != nil {
		return false
	}
	if stayDate.Before(from) {
		return false
	}
	if validTo == nil {
		return true
	}
	to, err := time.Parse("2006-01-02", *validTo)
	if err != nil {
		return false
	}
	return !stayDate.After(to)
}

func totalGuests(input model.BookingInput) int {
	if len(input.Guests) > 0 {
		return len(input.Guests)
	}
	return input.Adults + input.Children
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func getFloat(params map[string]any, key string) (float64, error) {
	value, ok := params[key]
	if !ok {
		return 0, fmt.Errorf("missing %s", key)
	}
	number, ok := value.(float64)
	if !ok {
		return 0, fmt.Errorf("%s must be a number", key)
	}
	return number, nil
}

func getInt(params map[string]any, key string) (int, error) {
	value, err := getFloat(params, key)
	if err != nil {
		return 0, err
	}
	return int(value), nil
}

func getString(params map[string]any, key string) (string, error) {
	value, ok := params[key]
	if !ok {
		return "", fmt.Errorf("missing %s", key)
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return text, nil
}

func getStringSlice(params map[string]any, key string) ([]string, error) {
	value, ok := params[key]
	if !ok {
		return nil, fmt.Errorf("missing %s", key)
	}
	raw, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s must contain only strings", key)
		}
		result = append(result, text)
	}
	return result, nil
}

func matchTierAmount(tiers []any, nights int) (float64, error) {
	for _, rawTier := range tiers {
		tier, ok := rawTier.(map[string]any)
		if !ok {
			return 0, errors.New("tier must be an object")
		}

		minNights := 0
		if rawMin, ok := tier["min_nights"]; ok {
			min, ok := rawMin.(float64)
			if !ok {
				return 0, errors.New("tier min_nights must be a number")
			}
			minNights = int(min)
		}

		if nights < minNights {
			continue
		}

		if rawMax, ok := tier["max_nights"]; ok {
			max, ok := rawMax.(float64)
			if !ok {
				return 0, errors.New("tier max_nights must be a number")
			}
			if nights >= int(max) {
				continue
			}
		}

		amount, ok := tier["amount"].(float64)
		if !ok {
			return 0, errors.New("tier amount must be a number")
		}
		return amount, nil
	}

	return 0, fmt.Errorf("no matching tier for %d nights", nights)
}

func parseMMDD(value string) (time.Month, int, error) {
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid mm-dd value %q", value)
	}
	t, err := time.Parse("01-02", value)
	if err != nil {
		return 0, 0, err
	}
	return t.Month(), t.Day(), nil
}
