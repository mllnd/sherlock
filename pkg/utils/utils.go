package utils

import "math"

// Round rounds a float64 to n decimal places
func Round(num float64, places int) float64 {
	multiplier := math.Pow(10, float64(places))
	return math.Round(num*multiplier) / multiplier
}
