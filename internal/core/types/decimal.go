package types

import "github.com/shopspring/decimal"

type Decimal = decimal.Decimal
type NullDecimal = decimal.NullDecimal

var (
	NewDecimalFromFloat  = decimal.NewFromFloat
	NewDecimalFromInt    = decimal.NewFromInt
	NewDecimalFromString = decimal.NewFromString
	Zero                 = decimal.Zero
)

func NewDecimal(v string) (Decimal, error) {
	return decimal.NewFromString(v)
}

func MustDecimal(v string) Decimal {
	return decimal.RequireFromString(v)
}
