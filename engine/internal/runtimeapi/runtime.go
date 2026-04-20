package runtimeapi

import (
	"errors"
	"fmt"

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

func resolveRegistry(inline *model.KindRegistry, fallback model.KindRegistry) (model.KindRegistry, error) {
	if inline != nil {
		return *inline, nil
	}
	if len(fallback.Calculations) == 0 && len(fallback.Predicates) == 0 {
		return model.KindRegistry{}, errNoRegistry
	}
	return fallback, nil
}
