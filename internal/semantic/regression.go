package semantic

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/SamuelSupe/contextGate/internal/model"
)

type RegressionCase struct {
	Name           string          `json:"name"`
	ParametersJSON string          `json:"parameters_json"`
	MinRows        *int            `json:"min_rows,omitempty"`
	MaxRows        *int            `json:"max_rows,omitempty"`
	Columns        []model.Column  `json:"columns,omitempty"`
	Values         []ExpectedValue `json:"values,omitempty"`
}

type ExpectedValue struct {
	Pointer      string `json:"pointer"`
	ExpectedJSON string `json:"expected_json"`
}

type CaseResult struct {
	Name      string `json:"name"`
	Passed    bool   `json:"passed"`
	ErrorCode string `json:"error_code,omitempty"`
	Rows      int    `json:"rows"`
	ElapsedMS int64  `json:"elapsed_ms"`
}

type TrialReport struct {
	Passed bool         `json:"passed"`
	Cases  []CaseResult `json:"cases"`
}

// Regression expectations gate publication without changing executable query
// versions or invalidating ongoing calls when only a test contract changes.
func EvidenceDefinition(t Template) string {
	if len(t.Tests) == 0 {
		return Definition(t)
	}
	b, _ := json.Marshal(struct {
		Definition string
		Example    string
		Tests      []RegressionCase
	}{Definition(t), t.ExampleJSON, t.Tests})
	hash := sha256.Sum256(b)
	return "regression:" + hex.EncodeToString(hash[:])
}

func ValidateRegression(t Template) error {
	if len(t.Tests) > 10 {
		return invalid("At most 10 regression cases per template")
	}
	names := map[string]bool{"Example": true}
	for _, c := range t.Tests {
		if strings.TrimSpace(c.Name) == "" || len(c.Name) > 120 || names[c.Name] {
			return invalid("Regression cases need unique names of at most 120 bytes (Example is reserved)")
		}
		names[c.Name] = true
		if len(c.ParametersJSON) > 16<<10 || len(c.Columns) > 100 || len(c.Values) > 50 {
			return invalid("Regression case exceeds its size limit")
		}
		if c.MinRows != nil && (*c.MinRows < 0 || *c.MinRows > 10000) || c.MaxRows != nil && (*c.MaxRows < 0 || *c.MaxRows > 10000) || c.MinRows != nil && c.MaxRows != nil && *c.MinRows > *c.MaxRows {
			return invalid("Regression row bounds must be ordered and between 0 and 10000")
		}
		for _, column := range c.Columns {
			if column.Name == "" || len(column.Name) > 256 || len(column.Type) > 120 {
				return invalid("Expected columns require a name and bounded native type")
			}
		}
		if _, err := BindCase(t, c); err != nil {
			return err
		}
		for _, v := range c.Values {
			if len(v.Pointer) > 512 || len(v.ExpectedJSON) > 4096 {
				return invalid("Regression value expectation exceeds its size limit")
			}
			if _, err := pointerParts(v.Pointer); err != nil {
				return err
			}
			if _, err := Parse(v.ExpectedJSON); err != nil {
				return invalid("Expected value must be valid JSON")
			}
		}
	}
	return nil
}

func BindCase(t Template, c RegressionCase) (model.Query, error) {
	v, err := Parse(c.ParametersJSON)
	if err != nil {
		return model.Query{}, invalid("Regression parameters must be a JSON object")
	}
	params, ok := v.(map[string]any)
	if !ok {
		return model.Query{}, invalid("Regression parameters must be a JSON object")
	}
	return Bind(t, params, false)
}

func CheckRegression(c RegressionCase, result *model.Result) string {
	// Row bounds and scalar checks must never silently approve a partial result.
	if result.Truncated || result.NextCursor != "" {
		return "incomplete_result"
	}
	if c.MinRows != nil && result.RowCount < *c.MinRows || c.MaxRows != nil && result.RowCount > *c.MaxRows {
		return "row_count_mismatch"
	}
	for _, want := range c.Columns {
		found := false
		for _, got := range result.Columns {
			if got.Name == want.Name && (want.Type == "" || want.Type == got.Type) {
				found = true
				break
			}
		}
		if !found {
			return "column_mismatch"
		}
	}
	raw, err := json.Marshal(result.Data)
	if err != nil {
		return "unreadable_result"
	}
	data, err := Parse(string(raw))
	if err != nil {
		return "unreadable_result"
	}
	for _, want := range c.Values {
		parts, err := pointerParts(want.Pointer)
		if err != nil {
			return "invalid_expectation"
		}
		value := data
		for _, part := range parts {
			value, err = child(value, part)
			if err != nil {
				return "value_missing"
			}
		}
		expected, err := Parse(want.ExpectedJSON)
		if err != nil || !reflect.DeepEqual(value, expected) {
			return "value_mismatch"
		}
	}
	return ""
}
