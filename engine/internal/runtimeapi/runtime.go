package runtimeapi

import (
	"errors"
	"fmt"
	"slices"

	"github.com/odeva-labs/tax-conformance-kit/engine/internal/engine"
	"github.com/odeva-labs/tax-conformance-kit/engine/internal/model"
)

var errNoRegistry = errors.New("kind registry is required")

func Validate(request model.RuntimeValidateRequest, defaultRegistry model.KindRegistry) model.RuntimeValidateResponse {
	registry, err := resolveRegistry(request.KindRegistry, defaultRegistry)
	if err != nil {
		return model.RuntimeValidateResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: err.Error()},
		}
	}

	if err := engine.ValidateRuleSet(request.RuleSet, registry); err != nil {
		return model.RuntimeValidateResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: err.Error()},
		}
	}

	return model.RuntimeValidateResponse{
		APIVersion: model.RuntimeAPIVersion,
		OK:         true,
		RuleCount:  len(request.RuleSet.Rules),
	}
}

func Evaluate(request model.RuntimeEvaluateRequest, defaultRegistry model.KindRegistry) model.RuntimeEvaluateResponse {
	registry, err := resolveRegistry(request.KindRegistry, defaultRegistry)
	if err != nil {
		return model.RuntimeEvaluateResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: err.Error()},
		}
	}

	if err := engine.ValidateRuleSet(request.RuleSet, registry); err != nil {
		return model.RuntimeEvaluateResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: err.Error()},
		}
	}

	result, err := engine.Evaluate(request.BookingInput, request.RuleSet)
	if err != nil {
		return model.RuntimeEvaluateResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: err.Error()},
		}
	}

	return model.RuntimeEvaluateResponse{
		APIVersion: model.RuntimeAPIVersion,
		OK:         true,
		RuleCount:  len(request.RuleSet.Rules),
		Result:     &result,
	}
}

func EvaluateAssessment(request model.RuntimeEvaluateAssessmentRequest, defaultRegistry model.KindRegistry) model.RuntimeEvaluateAssessmentResponse {
	registry, err := resolveRegistry(request.KindRegistry, defaultRegistry)
	if err != nil {
		return model.RuntimeEvaluateAssessmentResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: err.Error()},
		}
	}

	if err := engine.ValidateRuleSet(request.RuleSet, registry); err != nil {
		return model.RuntimeEvaluateAssessmentResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: err.Error()},
		}
	}

	result, err := engine.EvaluateAssessment(request.AssessmentInput, request.RuleSet)
	if err != nil {
		return model.RuntimeEvaluateAssessmentResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: err.Error()},
		}
	}

	return model.RuntimeEvaluateAssessmentResponse{
		APIVersion: model.RuntimeAPIVersion,
		OK:         true,
		RuleCount:  len(request.RuleSet.Rules),
		Result:     &result,
	}
}

func ResolveEvaluate(request model.RuntimeResolveEvaluateRequest, defaultRegistry model.KindRegistry) model.RuntimeResolveEvaluateResponse {
	registry, err := resolveRegistry(request.KindRegistry, defaultRegistry)
	if err != nil {
		return model.RuntimeResolveEvaluateResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: err.Error()},
		}
	}

	resolved, err := engine.ResolveRuleSet(request.BookingInput, engine.ResolveRuleSetRequest{
		FixtureRoot: request.FixtureRoot,
		Domain:      request.Domain,
	})
	if err != nil {
		return model.RuntimeResolveEvaluateResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: err.Error()},
		}
	}

	if err := engine.ValidateRuleSet(resolved.RuleSet, registry); err != nil {
		return model.RuntimeResolveEvaluateResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: fmt.Sprintf("resolved ruleset %s: %s", resolved.Path, err.Error())},
		}
	}

	result, err := engine.Evaluate(request.BookingInput, resolved.RuleSet)
	if err != nil {
		return model.RuntimeResolveEvaluateResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: err.Error()},
		}
	}

	return model.RuntimeResolveEvaluateResponse{
		APIVersion:          model.RuntimeAPIVersion,
		OK:                  true,
		RuleCount:           len(resolved.RuleSet.Rules),
		ResolvedRuleSetID:   resolved.RuleSet.ID,
		ResolvedRuleSetPath: resolved.Path,
		Result:              &result,
	}
}

func ResolveEvaluateAssessment(request model.RuntimeResolveEvaluateAssessmentRequest, defaultRegistry model.KindRegistry) model.RuntimeResolveEvaluateAssessmentResponse {
	registry, err := resolveRegistry(request.KindRegistry, defaultRegistry)
	if err != nil {
		return model.RuntimeResolveEvaluateAssessmentResponse{
			APIVersion: model.RuntimeAPIVersion,
			OK:         false,
			Error:      &model.RuntimeError{Message: err.Error()},
		}
	}

	type assessmentGroup struct {
		resolved engine.ResolvedRuleSet
		bookings []model.BookingInput
	}

	grouped := map[string]*assessmentGroup{}
	for _, booking := range request.AssessmentInput.Bookings {
		resolved, err := engine.ResolveRuleSet(booking, engine.ResolveRuleSetRequest{
			FixtureRoot: request.FixtureRoot,
			Domain:      request.Domain,
		})
		if err != nil {
			return model.RuntimeResolveEvaluateAssessmentResponse{
				APIVersion: model.RuntimeAPIVersion,
				OK:         false,
				Error:      &model.RuntimeError{Message: err.Error()},
			}
		}

		group, ok := grouped[resolved.Path]
		if !ok {
			group = &assessmentGroup{resolved: resolved}
			grouped[resolved.Path] = group
		}
		group.bookings = append(group.bookings, booking)
	}

	paths := make([]string, 0, len(grouped))
	for path := range grouped {
		paths = append(paths, path)
	}
	slices.Sort(paths)

	results := make([]model.ResolvedAssessmentResult, 0, len(paths))
	totalBookingTax := 0.0
	totalAssessmentTax := 0.0
	for _, path := range paths {
		group := grouped[path]
		if err := engine.ValidateRuleSet(group.resolved.RuleSet, registry); err != nil {
			return model.RuntimeResolveEvaluateAssessmentResponse{
				APIVersion: model.RuntimeAPIVersion,
				OK:         false,
				Error:      &model.RuntimeError{Message: fmt.Sprintf("resolved ruleset %s: %s", group.resolved.Path, err.Error())},
			}
		}

		result, err := engine.EvaluateAssessment(model.AssessmentInput{
			PeriodStart: request.AssessmentInput.PeriodStart,
			PeriodEnd:   request.AssessmentInput.PeriodEnd,
			Bookings:    group.bookings,
		}, group.resolved.RuleSet)
		if err != nil {
			return model.RuntimeResolveEvaluateAssessmentResponse{
				APIVersion: model.RuntimeAPIVersion,
				OK:         false,
				Error:      &model.RuntimeError{Message: err.Error()},
			}
		}

		results = append(results, model.ResolvedAssessmentResult{
			ResolvedRuleSetID:   group.resolved.RuleSet.ID,
			ResolvedRuleSetPath: group.resolved.Path,
			RuleCount:           len(group.resolved.RuleSet.Rules),
			BookingCount:        len(group.bookings),
			Result:              result,
		})
		totalBookingTax += result.TotalBookingTax
		totalAssessmentTax += result.TotalAssessmentTax
	}

	return model.RuntimeResolveEvaluateAssessmentResponse{
		APIVersion:          model.RuntimeAPIVersion,
		OK:                  true,
		GroupCount:          len(results),
		TotalBookingTax:     totalBookingTax,
		TotalAssessmentTax:  totalAssessmentTax,
		ResolvedAssessments: results,
	}
}

func resolveRegistry(inline *model.KindRegistry, fallback model.KindRegistry) (model.KindRegistry, error) {
	if inline != nil {
		return *inline, nil
	}
	if len(fallback.Calculations) == 0 && len(fallback.Predicates) == 0 {
		return model.KindRegistry{}, errNoRegistry
	}
	return fallback, nil
}
