// Package converters provides custom type converters.
package converters

import (
	"time"

	"github.com/mickamy/automapper"
)

func init() {
	// time.Time -> string (no error)
	automapper.RegisterTo[time.Time, string](TimeToRFC3339)

	// string -> time.Time (returns error on invalid format)
	automapper.RegisterFromE[string, time.Time](RFC3339ToTime)
}

// TimeToRFC3339 converts time.Time to RFC3339 string.
func TimeToRFC3339(t time.Time) string {
	return t.Format(time.RFC3339)
}

// RFC3339ToTime converts RFC3339 string to time.Time.
// Returns error if the string is not valid RFC3339 format.
func RFC3339ToTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
