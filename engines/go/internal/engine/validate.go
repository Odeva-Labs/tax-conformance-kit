package engine

import (
	"fmt"

	"github.com/odeva-labs/tax-conformance-kit/engines/go/internal/model"
)

func ValidateRuleSet(rs model.RuleSet, registry model.KindRegistry) error {
	if rs.Jurisdiction.CountryCode == "" {
		return fmt.Errorf("jurisdiction.country_code is required")
	}
	if len(rs.Rules) == 0 {
		return fmt.Errorf("rules cannot be empty")
	}

	for _, rule := range rs.Rules {
		if rule.ID == "" {
			return fmt.Errorf("rule id is required")
		}
		if rule.MunicipalityCode == "" {
			return fmt.Errorf("rule municipality_code is required")
		}
		if rule.ValidFrom == "" {
			return fmt.Errorf("rule valid_from is required")
		}
		if rule.Calculation.Kind == "" {
			return fmt.Errorf("rule %s calculation.kind is required", rule.ID)
		}
		calculationEntry, ok := registry.Calculations[rule.Calculation.Kind]
		if !ok {
			return fmt.Errorf("unknown calculation kind %s", rule.Calculation.Kind)
		}
		if err := validateParamsAgainstSchema(rule.Calculation.Params, calculationEntry.ParamsSchema, "rule "+rule.ID+" calculation.params"); err != nil {
			return err
		}
		for _, predicate := range rule.Predicates {
			if predicate.Kind == "" {
				return fmt.Errorf("rule %s predicate kind is required", rule.ID)
			}
			predicateEntry, ok := registry.Predicates[predicate.Kind]
			if !ok {
				return fmt.Errorf("unknown predicate kind %s", predicate.Kind)
			}
			if err := validateParamsAgainstSchema(predicate.Params, predicateEntry.ParamsSchema, "rule "+rule.ID+" predicate "+predicate.Kind+" params"); err != nil {
				return err
			}
		}
		for _, exemption := range rule.Exemptions {
			if exemption.Kind == "" {
				return fmt.Errorf("rule %s exemption kind is required", rule.ID)
			}
			exemptionEntry, ok := registry.Predicates[exemption.Kind]
			if !ok {
				return fmt.Errorf("unknown predicate kind %s", exemption.Kind)
			}
			if err := validateParamsAgainstSchema(exemption.Params, exemptionEntry.ParamsSchema, "rule "+rule.ID+" exemption "+exemption.Kind+" params"); err != nil {
				return err
			}
		}
	}

	return nil
}
