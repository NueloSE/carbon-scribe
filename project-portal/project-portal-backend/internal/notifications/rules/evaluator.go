package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"carbon-scribe/project-portal/project-portal-backend/internal/notifications"
)

// Evaluator evaluates rule conditions
type Evaluator struct{}

// NewEvaluator creates a new condition evaluator
func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// EvaluateCondition evaluates a single condition against data
func (e *Evaluator) EvaluateCondition(condition notifications.RuleCondition, data map[string]interface{}) (bool, error) {
	switch condition.Type {
	case "threshold":
		return e.evaluateThreshold(condition, data)
	case "rate_of_change":
		return e.evaluateRateOfChange(condition, data)
	case "pattern":
		return e.evaluatePattern(condition, data)
	default:
		return false, fmt.Errorf("unknown condition type: %s", condition.Type)
	}
}

// evaluateThreshold evaluates a threshold-based condition
func (e *Evaluator) evaluateThreshold(condition notifications.RuleCondition, data map[string]interface{}) (bool, error) {
	fieldValue, err := e.getFieldValue(condition.Field, data)
	if err != nil {
		return false, err
	}

	numValue, err := e.toFloat64(fieldValue)
	if err != nil {
		return false, fmt.Errorf("field %s is not numeric: %w", condition.Field, err)
	}

	threshold, err := e.toFloat64(condition.Value)
	if err != nil {
		return false, fmt.Errorf("threshold value is not numeric: %w", err)
	}

	return e.compare(numValue, condition.Operator, threshold)
}

// evaluateRateOfChange evaluates a rate-of-change condition
func (e *Evaluator) evaluateRateOfChange(condition notifications.RuleCondition, data map[string]interface{}) (bool, error) {
	// Rate of change requires current and previous values
	currentField := condition.Field
	previousField := condition.Field + "_previous"

	currentValue, err := e.getFieldValue(currentField, data)
	if err != nil {
		return false, err
	}

	previousValue, err := e.getFieldValue(previousField, data)
	if err != nil {
		// If no previous value, we can't calculate rate of change
		return false, nil
	}

	current, err := e.toFloat64(currentValue)
	if err != nil {
		return false, err
	}

	previous, err := e.toFloat64(previousValue)
	if err != nil {
		return false, err
	}

	// Calculate rate of change as percentage
	var rateOfChange float64
	if previous != 0 {
		rateOfChange = ((current - previous) / previous) * 100
	} else if current != 0 {
		rateOfChange = 100 // 100% increase from 0
	}

	threshold, err := e.toFloat64(condition.Value)
	if err != nil {
		return false, err
	}

	return e.compare(rateOfChange, condition.Operator, threshold)
}

// evaluatePattern evaluates a pattern-based condition
func (e *Evaluator) evaluatePattern(condition notifications.RuleCondition, data map[string]interface{}) (bool, error) {
	fieldValue, err := e.getFieldValue(condition.Field, data)
	if err != nil {
		return false, err
	}

	strValue := fmt.Sprintf("%v", fieldValue)

	switch condition.Operator {
	case "matches":
		matched, err := regexp.MatchString(condition.Pattern, strValue)
		if err != nil {
			return false, fmt.Errorf("invalid regex pattern: %w", err)
		}
		return matched, nil
	case "not_matches":
		matched, err := regexp.MatchString(condition.Pattern, strValue)
		if err != nil {
			return false, fmt.Errorf("invalid regex pattern: %w", err)
		}
		return !matched, nil
	case "contains":
		return strings.Contains(strValue, condition.Pattern), nil
	case "not_contains":
		return !strings.Contains(strValue, condition.Pattern), nil
	case "starts_with":
		return strings.HasPrefix(strValue, condition.Pattern), nil
	case "ends_with":
		return strings.HasSuffix(strValue, condition.Pattern), nil
	default:
		return false, fmt.Errorf("unknown pattern operator: %s", condition.Operator)
	}
}

// getFieldValue retrieves a field value from nested data using dot notation
func (e *Evaluator) getFieldValue(field string, data map[string]interface{}) (interface{}, error) {
	parts := strings.Split(field, ".")
	current := interface{}(data)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("field not found: %s", field)
			}
			current = val
		default:
			return nil, fmt.Errorf("cannot traverse non-map at %s", part)
		}
	}

	return current, nil
}

// toFloat64 converts a value to float64
func (e *Evaluator) toFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

// compare compares two values using the specified operator
func (e *Evaluator) compare(value float64, operator string, threshold float64) (bool, error) {
	switch operator {
	case "gt", ">":
		return value > threshold, nil
	case "gte", ">=":
		return value >= threshold, nil
	case "lt", "<":
		return value < threshold, nil
	case "lte", "<=":
		return value <= threshold, nil
	case "eq", "==":
		return value == threshold, nil
	case "neq", "!=":
		return value != threshold, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", operator)
	}
}

// ValidateCondition validates a condition structure
func (e *Evaluator) ValidateCondition(condition notifications.RuleCondition) error {
	if condition.Type == "" {
		return fmt.Errorf("condition type is required")
	}

	validTypes := []string{"threshold", "rate_of_change", "pattern"}
	typeValid := false
	for _, t := range validTypes {
		if condition.Type == t {
			typeValid = true
			break
		}
	}
	if !typeValid {
		return fmt.Errorf("invalid condition type: %s", condition.Type)
	}

	if condition.Field == "" {
		return fmt.Errorf("condition field is required")
	}

	if condition.Operator == "" {
		return fmt.Errorf("condition operator is required")
	}

	switch condition.Type {
	case "threshold", "rate_of_change":
		validOps := []string{"gt", ">", "gte", ">=", "lt", "<", "lte", "<=", "eq", "==", "neq", "!="}
		opValid := false
		for _, op := range validOps {
			if condition.Operator == op {
				opValid = true
				break
			}
		}
		if !opValid {
			return fmt.Errorf("invalid operator for threshold: %s", condition.Operator)
		}
		if condition.Value == nil {
			return fmt.Errorf("threshold value is required")
		}
	case "pattern":
		validOps := []string{"matches", "not_matches", "contains", "not_contains", "starts_with", "ends_with"}
		opValid := false
		for _, op := range validOps {
			if condition.Operator == op {
				opValid = true
				break
			}
		}
		if !opValid {
			return fmt.Errorf("invalid operator for pattern: %s", condition.Operator)
		}
		if condition.Pattern == "" {
			return fmt.Errorf("pattern is required for pattern conditions")
		}
	}

	return nil
}
