package pricing

import "github.com/shopspring/decimal"

func CeilMoney(value float64) float64 {
	result, _ := decimal.NewFromFloat(value).Mul(decimal.NewFromInt(100)).Ceil().Div(decimal.NewFromInt(100)).Float64()
	return result
}
