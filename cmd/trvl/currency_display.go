package main

import (
	"context"
	"math"

	"github.com/MikkoParkkola/trvl/internal/destinations"
)

var convertDisplayCurrencyFunc = destinations.ConvertCurrency

func convertRoundedDisplayAmounts(
	ctx context.Context,
	from, to string,
	decimals int,
	amounts ...*float64,
) string {
	currency := from
	for _, amount := range amounts {
		if amount == nil || *amount <= 0 {
			continue
		}
		converted, convertedCurrency := convertDisplayCurrencyFunc(ctx, *amount, from, to)
		*amount = roundDisplayAmount(converted, decimals)
		currency = convertedCurrency
	}
	return currency
}

func roundDisplayAmount(amount float64, decimals int) float64 {
	if decimals <= 0 {
		return math.Round(amount)
	}
	factor := math.Pow(10, float64(decimals))
	return math.Round(amount*factor) / factor
}
