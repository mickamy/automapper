// Package converters provides custom type converters.
package converters

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mickamy/automapper"
	"github.com/mickamy/automapper/testdata/advanced/model"
)

func init() {
	// Register time.Time <-> int64 converters
	automapper.RegisterTo[time.Time, int64](TimeToUnix)
	automapper.RegisterFrom[time.Time, int64](UnixToTime)

	// Register OrderStatus <-> int32 converters
	automapper.RegisterTo[model.OrderStatus, int32](StatusToInt32)
	automapper.RegisterFrom[model.OrderStatus, int32](Int32ToStatus)

	// Named converters for price formatting
	// map:",conv=priceToInt" / map:",conv=priceFromInt" で参照される
	automapper.RegisterToNamed[string, int64]("priceToInt", PriceStringToCents)
	automapper.RegisterFromNamedE[string, int64]("priceFromInt", CentsToPriceString)
}

// TimeToUnix converts time.Time to Unix timestamp.
func TimeToUnix(t time.Time) int64 {
	return t.Unix()
}

// UnixToTime converts Unix timestamp to time.Time.
func UnixToTime(ts int64) time.Time {
	return time.Unix(ts, 0)
}

// StatusToInt32 converts OrderStatus to int32.
func StatusToInt32(s model.OrderStatus) int32 {
	return int32(s) //nolint:gosec // test code, overflow acceptable
}

// Int32ToStatus converts int32 to OrderStatus.
func Int32ToStatus(i int32) model.OrderStatus {
	return model.OrderStatus(i)
}

// PriceStringToCents converts "$19.99" to 1999 (cents).
func PriceStringToCents(s string) int64 {
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}

	return int64(f*100 + 0.5) // Round to nearest
}

// CentsToPriceString converts 1999 (cents) to "$19.99".
func CentsToPriceString(cents int64) (string, error) {
	if cents < 0 {
		return "", fmt.Errorf("negative price: %d", cents)
	}

	return fmt.Sprintf("$%.2f", float64(cents)/100), nil
}
