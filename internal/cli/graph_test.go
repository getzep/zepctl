package cli

import (
	"testing"

	"github.com/getzep/zep-go/v3"
)

func TestParsePropertyFilter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *zep.PropertyFilter
		wantErr  bool
	}{
		{
			name:  "equals string",
			input: "status:=:active",
			expected: &zep.PropertyFilter{
				PropertyName:       "status",
				ComparisonOperator: zep.ComparisonOperatorEquals,
				PropertyValue:      "active",
			},
		},
		{
			name:  "equals with double equals",
			input: "status:==:active",
			expected: &zep.PropertyFilter{
				PropertyName:       "status",
				ComparisonOperator: zep.ComparisonOperatorEquals,
				PropertyValue:      "active",
			},
		},
		{
			name:  "not equals",
			input: "status:<>:inactive",
			expected: &zep.PropertyFilter{
				PropertyName:       "status",
				ComparisonOperator: zep.ComparisonOperatorNotEquals,
				PropertyValue:      "inactive",
			},
		},
		{
			name:  "not equals with !=",
			input: "status:!=:inactive",
			expected: &zep.PropertyFilter{
				PropertyName:       "status",
				ComparisonOperator: zep.ComparisonOperatorNotEquals,
				PropertyValue:      "inactive",
			},
		},
		{
			name:  "greater than integer",
			input: "age:>:30",
			expected: &zep.PropertyFilter{
				PropertyName:       "age",
				ComparisonOperator: zep.ComparisonOperatorGreaterThan,
				PropertyValue:      int64(30),
			},
		},
		{
			name:  "less than integer",
			input: "count:<:100",
			expected: &zep.PropertyFilter{
				PropertyName:       "count",
				ComparisonOperator: zep.ComparisonOperatorLessThan,
				PropertyValue:      int64(100),
			},
		},
		{
			name:  "greater than or equal",
			input: "score:>=:75",
			expected: &zep.PropertyFilter{
				PropertyName:       "score",
				ComparisonOperator: zep.ComparisonOperatorGreaterThanEqual,
				PropertyValue:      int64(75),
			},
		},
		{
			name:  "less than or equal float",
			input: "rating:<=:4.5",
			expected: &zep.PropertyFilter{
				PropertyName:       "rating",
				ComparisonOperator: zep.ComparisonOperatorLessThanEqual,
				PropertyValue:      4.5,
			},
		},
		{
			name:  "boolean true",
			input: "active:=:true",
			expected: &zep.PropertyFilter{
				PropertyName:       "active",
				ComparisonOperator: zep.ComparisonOperatorEquals,
				PropertyValue:      true,
			},
		},
		{
			name:  "boolean false",
			input: "deleted:=:false",
			expected: &zep.PropertyFilter{
				PropertyName:       "deleted",
				ComparisonOperator: zep.ComparisonOperatorEquals,
				PropertyValue:      false,
			},
		},
		{
			name:  "IS NULL",
			input: "deleted_at:IS NULL",
			expected: &zep.PropertyFilter{
				PropertyName:       "deleted_at",
				ComparisonOperator: zep.ComparisonOperatorIsNull,
			},
		},
		{
			name:  "IS NOT NULL",
			input: "verified_at:IS NOT NULL",
			expected: &zep.PropertyFilter{
				PropertyName:       "verified_at",
				ComparisonOperator: zep.ComparisonOperatorIsNotNull,
			},
		},
		{
			name:    "invalid format - missing value",
			input:   "status:=",
			wantErr: true,
		},
		{
			name:    "invalid format - missing parts",
			input:   "status",
			wantErr: true,
		},
		{
			name:    "invalid operator",
			input:   "status:LIKE:test",
			wantErr: true,
		},
		{
			name:    "empty property name for IS NULL",
			input:   ":IS NULL",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parsePropertyFilter(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result.PropertyName != tt.expected.PropertyName {
				t.Errorf("PropertyName = %v, want %v", result.PropertyName, tt.expected.PropertyName)
			}
			if result.ComparisonOperator != tt.expected.ComparisonOperator {
				t.Errorf("ComparisonOperator = %v, want %v", result.ComparisonOperator, tt.expected.ComparisonOperator)
			}
			if result.PropertyValue != tt.expected.PropertyValue {
				t.Errorf("PropertyValue = %v (%T), want %v (%T)", result.PropertyValue, result.PropertyValue, tt.expected.PropertyValue, tt.expected.PropertyValue)
			}
		})
	}
}

func TestParseDateFilter(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantField string
		wantOp    zep.ComparisonOperator
		wantDate  *string
		wantErr   bool
	}{
		{
			name:      "created_at greater than",
			input:     "created_at:>:2024-01-01",
			wantField: "created_at",
			wantOp:    zep.ComparisonOperatorGreaterThan,
			wantDate:  strPtr("2024-01-01"),
		},
		{
			name:      "valid_at less than",
			input:     "valid_at:<:2024-12-31",
			wantField: "valid_at",
			wantOp:    zep.ComparisonOperatorLessThan,
			wantDate:  strPtr("2024-12-31"),
		},
		{
			name:      "invalid_at equals",
			input:     "invalid_at:=:2024-06-15",
			wantField: "invalid_at",
			wantOp:    zep.ComparisonOperatorEquals,
			wantDate:  strPtr("2024-06-15"),
		},
		{
			name:      "expired_at not equals",
			input:     "expired_at:<>:2024-03-01",
			wantField: "expired_at",
			wantOp:    zep.ComparisonOperatorNotEquals,
			wantDate:  strPtr("2024-03-01"),
		},
		{
			name:      "created_at IS NULL",
			input:     "created_at:IS NULL",
			wantField: "created_at",
			wantOp:    zep.ComparisonOperatorIsNull,
			wantDate:  nil,
		},
		{
			name:      "valid_at IS NOT NULL",
			input:     "valid_at:IS NOT NULL",
			wantField: "valid_at",
			wantOp:    zep.ComparisonOperatorIsNotNull,
			wantDate:  nil,
		},
		{
			name:    "unknown field",
			input:   "unknown_field:>:2024-01-01",
			wantErr: true,
		},
		{
			name:    "invalid format - missing date",
			input:   "created_at:>",
			wantErr: true,
		},
		{
			name:    "invalid operator",
			input:   "created_at:BETWEEN:2024-01-01",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &zep.SearchFilters{}
			err := parseDateFilter(tt.input, sf)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			filters := getDateFiltersForField(sf, tt.wantField)
			if len(filters) != 1 || len(filters[0]) != 1 {
				t.Errorf("expected 1 filter group with 1 filter, got %v", filters)
				return
			}

			df := filters[0][0]
			if df.ComparisonOperator != tt.wantOp {
				t.Errorf("ComparisonOperator = %v, want %v", df.ComparisonOperator, tt.wantOp)
			}
			assertDateEqual(t, df.Date, tt.wantDate)
		})
	}
}

func getDateFiltersForField(sf *zep.SearchFilters, field string) [][]*zep.DateFilter {
	switch field {
	case "created_at":
		return sf.CreatedAt
	case "valid_at":
		return sf.ValidAt
	case "invalid_at":
		return sf.InvalidAt
	case "expired_at":
		return sf.ExpiredAt
	default:
		return nil
	}
}

func assertDateEqual(t *testing.T, got, want *string) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Errorf("Date = %v, want nil", *got)
		}
		return
	}
	if got == nil {
		t.Errorf("Date = nil, want %v", *want)
		return
	}
	if *got != *want {
		t.Errorf("Date = %v, want %v", *got, *want)
	}
}

func TestParseComparisonOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected zep.ComparisonOperator
		wantErr  bool
	}{
		{"=", zep.ComparisonOperatorEquals, false},
		{"==", zep.ComparisonOperatorEquals, false},
		{"<>", zep.ComparisonOperatorNotEquals, false},
		{"!=", zep.ComparisonOperatorNotEquals, false},
		{">", zep.ComparisonOperatorGreaterThan, false},
		{"<", zep.ComparisonOperatorLessThan, false},
		{">=", zep.ComparisonOperatorGreaterThanEqual, false},
		{"<=", zep.ComparisonOperatorLessThanEqual, false},
		{"IS NULL", zep.ComparisonOperatorIsNull, false},
		{"IS NOT NULL", zep.ComparisonOperatorIsNotNull, false},
		{"LIKE", "", true},
		{"IN", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseComparisonOperator(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
