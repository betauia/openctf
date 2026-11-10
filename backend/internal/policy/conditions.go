package policy

import (
	"encoding/json"
	"fmt"
	"strings"
)

var validOperators = map[string]bool{
	"Equals": true, "NotEquals": true, "In": true, "NotIn": true, "LessThan": true, "GreaterThan": true,
}

func evaluateConditions(conds []Condition, req Request) (bool, error) {
	for _, c := range conds {
		ok, err := evaluateCondition(c, req)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

func evaluateCondition(c Condition, req Request) (bool, error) {
	left, found := resolveKey(c.Key, req)
	if !found {
		return false, nil
	}
	right := c.Value
	if s, ok := right.(string); ok && strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		v, ok := resolveKey(s[2:len(s)-1], req)
		if !ok {
			return false, nil
		}
		right = v
	}

	switch c.Operator {
	case "Equals":
		return equals(left, right), nil
	case "NotEquals":
		return !equals(left, right), nil
	case "In", "NotIn":
		list, ok := toList(right)
		if !ok {
			return false, fmt.Errorf("operator %s needs a list value (key %q)", c.Operator, c.Key)
		}
		found := false
		for _, item := range list {
			if equals(left, item) {
				found = true
				break
			}
		}
		return found == (c.Operator == "In"), nil
	case "LessThan", "GreaterThan":
		l, ok1 := toFloat(left)
		r, ok2 := toFloat(right)
		if !ok1 || !ok2 {
			return false, fmt.Errorf("operator %s needs numeric values (key %q)", c.Operator, c.Key)
		}
		if c.Operator == "LessThan" {
			return l < r, nil
		}
		return l > r, nil
	default:
		return false, fmt.Errorf("unknown operator %q", c.Operator)
	}
}

// resolveKey looks up "subject.x", "resource.x", "context.x" or "request.action" in the request.
func resolveKey(key string, req Request) (any, bool) {
	scope, attr, ok := strings.Cut(key, ".")
	if !ok {
		return nil, false
	}
	switch scope {
	case "subject":
		if attr == "id" {
			return req.Subject.ID, true
		}
		v, ok := req.Subject.Attributes[attr]
		return v, ok
	case "resource":
		switch attr {
		case "id":
			return req.Resource.ID, true
		case "path":
			return req.Resource.Path, true
		}
		v, ok := req.Resource.Attributes[attr]
		return v, ok
	case "context":
		v, ok := req.Context[attr]
		return v, ok
	case "request":
		if attr == "action" {
			return req.Action, true
		}
	}
	return nil, false
}

func equals(a, b any) bool {
	if af, ok := toFloat(a); ok {
		if bf, ok := toFloat(b); ok {
			return af == bf
		}
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func toList(v any) ([]any, bool) {
	switch l := v.(type) {
	case []any:
		return l, true
	case []string:
		out := make([]any, len(l))
		for i, s := range l {
			out[i] = s
		}
		return out, true
	}
	return nil, false
}
