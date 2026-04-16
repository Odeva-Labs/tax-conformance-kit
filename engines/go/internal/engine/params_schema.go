package engine

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

func validateParamsAgainstSchema(value any, schema map[string]any, path string) error {
	schemaType, _ := schema["type"].(string)

	switch schemaType {
	case "object":
		obj, err := normalizeObject(value)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		required := toStringSlice(schema["required"])
		for _, key := range required {
			if _, ok := obj[key]; !ok {
				return fmt.Errorf("%s.%s is required", path, key)
			}
		}

		properties := map[string]any{}
		if rawProps, ok := schema["properties"].(map[string]any); ok {
			properties = rawProps
		}
		additionalAllowed := true
		if rawAdditional, ok := schema["additionalProperties"].(bool); ok {
			additionalAllowed = rawAdditional
		}

		for key, raw := range obj {
			propSchema, ok := properties[key]
			if !ok {
				if !additionalAllowed {
					return fmt.Errorf("%s.%s is not allowed", path, key)
				}
				continue
			}
			childSchema, ok := propSchema.(map[string]any)
			if !ok {
				return fmt.Errorf("%s.%s has invalid schema", path, key)
			}
			if err := validateParamsAgainstSchema(raw, childSchema, path+"."+key); err != nil {
				return err
			}
		}
		return nil

	case "array":
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s must be an array", path)
		}
		if minItems, ok := getSchemaNumber(schema["minItems"]); ok && len(items) < int(minItems) {
			return fmt.Errorf("%s must contain at least %d items", path, int(minItems))
		}
		itemSchema, _ := schema["items"].(map[string]any)
		for i, item := range items {
			if itemSchema == nil {
				continue
			}
			if err := validateParamsAgainstSchema(item, itemSchema, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		return nil

	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s must be a string", path)
		}
		if enumValues := toStringSlice(schema["enum"]); len(enumValues) > 0 {
			if !containsString(enumValues, text) {
				return fmt.Errorf("%s must be one of %s", path, strings.Join(enumValues, ", "))
			}
		}
		if pattern, ok := schema["pattern"].(string); ok {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return fmt.Errorf("%s has invalid regex pattern", path)
			}
			if !re.MatchString(text) {
				return fmt.Errorf("%s must match pattern %s", path, pattern)
			}
		}
		return nil

	case "number":
		number, ok := value.(float64)
		if !ok {
			return fmt.Errorf("%s must be a number", path)
		}
		if min, ok := getSchemaNumber(schema["minimum"]); ok && number < min {
			return fmt.Errorf("%s must be >= %v", path, min)
		}
		return nil

	case "integer":
		number, ok := value.(float64)
		if !ok {
			return fmt.Errorf("%s must be an integer", path)
		}
		if math.Trunc(number) != number {
			return fmt.Errorf("%s must be an integer", path)
		}
		if min, ok := getSchemaNumber(schema["minimum"]); ok && number < min {
			return fmt.Errorf("%s must be >= %v", path, min)
		}
		return nil

	default:
		return nil
	}
}

func normalizeObject(value any) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("must be an object")
	}
	return obj, nil
}

func getSchemaNumber(value any) (float64, bool) {
	number, ok := value.(float64)
	return number, ok
}

func toStringSlice(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			return nil
		}
		out = append(out, text)
	}
	return out
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
