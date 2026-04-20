package engine

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/odeva-labs/tax-conformance-kit/engine/internal/model"
)

type CalculationHandler func(params map[string]any, input model.BookingInput) (float64, error)
type PredicateHandler func(params map[string]any, input model.BookingInput) (bool, error)

var calculationHandlers = map[string]CalculationHandler{}
var predicateHandlers = map[string]PredicateHandler{}

func init() {
	RegisterCalculation("generic.per_night", func(params map[string]any, input model.BookingInput) (float64, error) {
		amount, err := getFloat(params, "amount")
		if err != nil {
			return 0, err
		}
		return amount * float64(input.Nights), nil
	})

	RegisterCalculation("generic.per_person_per_night", func(params map[string]any, input model.BookingInput) (float64, error) {
		amount, err := getFloat(params, "amount")
		if err != nil {
			return 0, err
		}
		return amount * float64(totalGuests(input)) * float64(input.Nights), nil
	})

	RegisterCalculation("generic.fixed_amount", func(params map[string]any, input model.BookingInput) (float64, error) {
		return getFloat(params, "amount")
	})

	RegisterCalculation("generic.percentage_of_base", func(params map[string]any, input model.BookingInput) (float64, error) {
		ratePct, err := getFloat(params, "rate_pct")
		if err != nil {
			return 0, err
		}

		base, err := getString(params, "base")
		if err != nil {
			return 0, err
		}

		switch base {
		case "accommodation_fee_exclusive_of_tax", "gross_booking_total", "subtotal":
			return (input.Subtotal * ratePct) / 100.0, nil
		default:
			return 0, fmt.Errorf("unsupported percentage base %q", base)
		}
	})

	RegisterCalculation("nl.tiered_by_stay_duration", func(params map[string]any, input model.BookingInput) (float64, error) {
		rawTiers, ok := params["tiers"].([]any)
		if !ok {
			return 0, fmt.Errorf("tiers must be an array")
		}
		return matchTierAmount(rawTiers, input.Nights)
	})

	RegisterPredicate("guest.resident_of_same_municipality", func(params map[string]any, input model.BookingInput) (bool, error) {
		if input.MainGuestMunicipalityCode == nil {
			return false, nil
		}
		return strings.EqualFold(*input.MainGuestMunicipalityCode, input.PropertyMunicipalityCode), nil
	})

	RegisterPredicate("guest.age_below", func(params map[string]any, input model.BookingInput) (bool, error) {
		maxAge, err := getInt(params, "max_age")
		if err != nil {
			return false, err
		}
		requiredCount := 1
		if _, ok := params["count"]; ok {
			requiredCount, err = getInt(params, "count")
			if err != nil {
				return false, err
			}
		}

		matches := 0
		for _, guest := range input.Guests {
			if guest.Age != nil && *guest.Age < maxAge {
				matches++
			}
		}
		return matches >= requiredCount, nil
	})

	RegisterPredicate("stay.purpose_in", func(params map[string]any, input model.BookingInput) (bool, error) {
		values, err := getStringSlice(params, "values")
		if err != nil {
			return false, err
		}
		return slices.Contains(values, input.StayPurpose), nil
	})

	RegisterPredicate("stay.wtza_care_institution", func(params map[string]any, input model.BookingInput) (bool, error) {
		return input.WTZACareInstitution, nil
	})

	RegisterPredicate("stay.coa_asylum_housing", func(params map[string]any, input model.BookingInput) (bool, error) {
		return input.COAAsylumHousing, nil
	})

	RegisterPredicate("stay.accommodation_brought_by", func(params map[string]any, input model.BookingInput) (bool, error) {
		value, err := getString(params, "value")
		if err != nil {
			return false, err
		}
		return input.AccommodationBroughtBy == value, nil
	})

	RegisterPredicate("stay.pricing_arrangement", func(params map[string]any, input model.BookingInput) (bool, error) {
		value, err := getString(params, "value")
		if err != nil {
			return false, err
		}
		return input.PricingArrangement == value, nil
	})

	RegisterPredicate("stay.supervised_minor_group", func(params map[string]any, input model.BookingInput) (bool, error) {
		minMinors, err := getInt(params, "min_minors")
		if err != nil {
			return false, err
		}
		minSupervisors, err := getInt(params, "min_supervisors")
		if err != nil {
			return false, err
		}
		supervisorMinAge, err := getInt(params, "supervisor_min_age")
		if err != nil {
			return false, err
		}

		if _, ok := params["allowed_purposes"]; ok {
			values, err := getStringSlice(params, "allowed_purposes")
			if err != nil {
				return false, err
			}
			if !slices.Contains(values, input.StayPurpose) {
				return false, nil
			}
		}

		minors := 0
		supervisors := 0
		for _, guest := range input.Guests {
			if guest.Age == nil {
				continue
			}
			if *guest.Age < 18 {
				minors++
			}
			if guest.Role == "supervisor" && *guest.Age >= supervisorMinAge {
				supervisors++
			}
		}

		return minors >= minMinors && supervisors >= minSupervisors, nil
	})

	RegisterPredicate("stay.seasonal_window", func(params map[string]any, input model.BookingInput) (bool, error) {
		start, err := getString(params, "start_mmdd")
		if err != nil {
			return false, err
		}
		end, err := getString(params, "end_mmdd")
		if err != nil {
			return false, err
		}

		startMonth, startDay, err := parseMMDD(start)
		if err != nil {
			return false, err
		}
		endMonth, endDay, err := parseMMDD(end)
		if err != nil {
			return false, err
		}

		stayDate, err := time.Parse("2006-01-02", input.StayDate)
		if err != nil {
			return false, err
		}
		stayMonthDay := int(stayDate.Month())*100 + stayDate.Day()
		startMonthDay := int(startMonth)*100 + startDay
		endMonthDay := int(endMonth)*100 + endDay

		if startMonthDay <= endMonthDay {
			return stayMonthDay >= startMonthDay && stayMonthDay <= endMonthDay, nil
		}

		return stayMonthDay >= startMonthDay || stayMonthDay <= endMonthDay, nil
	})

	RegisterPredicate("cross_tax.already_subject_to", func(params map[string]any, input model.BookingInput) (bool, error) {
		tax, err := getString(params, "tax")
		if err != nil {
			return false, err
		}
		return slices.Contains(input.AlreadySubjectTo, tax), nil
	})
}

func RegisterCalculation(kind string, handler CalculationHandler) {
	if _, exists := calculationHandlers[kind]; exists {
		panic("duplicate calculation handler for " + kind)
	}
	calculationHandlers[kind] = handler
}

func RegisterPredicate(kind string, handler PredicateHandler) {
	if _, exists := predicateHandlers[kind]; exists {
		panic("duplicate predicate handler for " + kind)
	}
	predicateHandlers[kind] = handler
}

func GetCalculation(kind string) (CalculationHandler, bool) {
	handler, ok := calculationHandlers[kind]
	return handler, ok
}

func GetPredicate(kind string) (PredicateHandler, bool) {
	handler, ok := predicateHandlers[kind]
	return handler, ok
}
