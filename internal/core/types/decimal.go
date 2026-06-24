// Package types provides shared domain types used across modules.
package types

import "github.com/shopspring/decimal"

// Decimal is an alias for shopspring/decimal, providing exact base-10 arithmetic
// for monetary values where float64 rounding errors are unacceptable.
type Decimal = decimal.Decimal

// NullDecimal is the nullable variant of Decimal.
type NullDecimal = decimal.NullDecimal

// Re-exported constructors from shopspring/decimal.
var (
	NewDecimalFromFloat  = decimal.NewFromFloat
	NewDecimalFromInt    = decimal.NewFromInt
	NewDecimalFromString = decimal.NewFromString
	Zero                 = decimal.Zero
)

// NewDecimal parses a string into a Decimal, returning an error on invalid input.
func NewDecimal(v string) (Decimal, error) {
	return decimal.NewFromString(v)
}

// MustDecimal parses a string into a Decimal, panicking on invalid input.
// Only use for compile-time-known constants.
func MustDecimal(v string) Decimal {
	return decimal.RequireFromString(v)
}
