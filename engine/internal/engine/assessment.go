package engine

import (
	"fmt"
	"time"

	"github.com/odeva-labs/tax-conformance-kit/engine/internal/model"
)

func EvaluateAssessment(input model.AssessmentInput, rs model.RuleSet) (model.AssessmentCaseResult, error) {
	periodStart, periodEnd, err := parseAssessmentPeriod(input)
	if err != nil {
		return model.AssessmentCaseResult{}, err
	}

	bookingResults := make([]model.EvaluationResult, 0, len(input.Bookings))
	totalBookingTax := 0.0
	for i, booking := range input.Bookings {
		stayDate, err := time.Parse("2006-01-02", booking.StayDate)
		if err != nil {
			return model.AssessmentCaseResult{}, fmt.Errorf("booking %d: invalid stay_date", i)
		}
		if stayDate.Before(periodStart) || stayDate.After(periodEnd) {
			return model.AssessmentCaseResult{}, fmt.Errorf("booking %d stay_date %s falls outside assessment period", i, booking.StayDate)
		}

		result, err := Evaluate(booking, rs)
		if err != nil {
			return model.AssessmentCaseResult{}, fmt.Errorf("booking %d: %w", i, err)
		}
		bookingResults = append(bookingResults, result)
		totalBookingTax += result.TotalTax
	}

	totalAssessmentTax := round2(totalBookingTax)
	if policy := rs.AssessmentPolicy; policy != nil && policy.MinimumAssessmentAmount != nil {
		if totalAssessmentTax < policy.MinimumAssessmentAmount.Amount {
			totalAssessmentTax = 0
		}
	}

	return model.AssessmentCaseResult{
		TotalBookingTax:    round2(totalBookingTax),
		TotalAssessmentTax: totalAssessmentTax,
		BookingResults:     bookingResults,
	}, nil
}

func parseAssessmentPeriod(input model.AssessmentInput) (time.Time, time.Time, error) {
	periodStart, err := time.Parse("2006-01-02", input.PeriodStart)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid period_start")
	}
	periodEnd, err := time.Parse("2006-01-02", input.PeriodEnd)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid period_end")
	}
	if periodEnd.Before(periodStart) {
		return time.Time{}, time.Time{}, fmt.Errorf("period_end must be on or after period_start")
	}
	return periodStart, periodEnd, nil
}
