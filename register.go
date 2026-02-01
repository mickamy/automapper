// Package automapper provides struct-to-struct mapping code generation.
// Users register custom converters that the CLI uses during code generation.
package automapper

// RegisterTo registers a converter function from type A to type B.
// The converter is called when generating mapping code where the source
// field is type A and target field is type B.
//
// Example:
//
//	func init() {
//	    automapper.RegisterTo[time.Time, *datepb.Date](ToDate)
//	}
func RegisterTo[A, B any](fn func(A) B) {
	// This function is a marker for the CLI to discover.
	// The actual implementation is a no-op at runtime.
}

// RegisterFrom registers a converter function from type B to type A.
// This is the reverse direction of RegisterTo.
//
// Example:
//
//	func init() {
//	    automapper.RegisterFrom[time.Time, *datepb.Date](FromDate)
//	}
func RegisterFrom[A, B any](fn func(B) A) {
	// This function is a marker for the CLI to discover.
	// The actual implementation is a no-op at runtime.
}

// RegisterToE registers a converter function from type A to type B that may return an error.
//
// Example:
//
//	func init() {
//	    automapper.RegisterToE[string, int](strconv.Atoi)
//	}
func RegisterToE[A, B any](fn func(A) (B, error)) {
	// This function is a marker for the CLI to discover.
	// The actual implementation is a no-op at runtime.
}

// RegisterFromE registers a converter function from type B to type A that may return an error.
//
// Example:
//
//	func init() {
//	    automapper.RegisterFromE[time.Time, *datepb.Date](FromDate)
//	}
func RegisterFromE[A, B any](fn func(B) (A, error)) {
	// This function is a marker for the CLI to discover.
	// The actual implementation is a no-op at runtime.
}

// RegisterToNamed registers a named converter function from type A to type B.
// Named converters can be referenced in struct tags using map:",conv=name".
//
// Example:
//
//	func init() {
//	    automapper.RegisterToNamed[time.Time, string]("rfc3339", TimeToRFC3339)
//	}
func RegisterToNamed[A, B any](name string, fn func(A) B) {
	// This function is a marker for the CLI to discover.
	// The actual implementation is a no-op at runtime.
}

// RegisterFromNamed registers a named converter function from type B to type A.
//
// Example:
//
//	func init() {
//	    automapper.RegisterFromNamed[time.Time, string]("rfc3339", RFC3339ToTime)
//	}
func RegisterFromNamed[A, B any](name string, fn func(B) A) {
	// This function is a marker for the CLI to discover.
	// The actual implementation is a no-op at runtime.
}

// RegisterToNamedE registers a named converter function from type A to type B that may return an error.
//
// Example:
//
//	func init() {
//	    automapper.RegisterToNamedE[time.Time, string]("rfc3339", TimeToRFC3339E)
//	}
func RegisterToNamedE[A, B any](name string, fn func(A) (B, error)) {
	// This function is a marker for the CLI to discover.
	// The actual implementation is a no-op at runtime.
}

// RegisterFromNamedE registers a named converter function from type B to type A that may return an error.
//
// Example:
//
//	func init() {
//	    automapper.RegisterFromNamedE[time.Time, string]("rfc3339", RFC3339ToTimeE)
//	}
func RegisterFromNamedE[A, B any](name string, fn func(B) (A, error)) {
	// This function is a marker for the CLI to discover.
	// The actual implementation is a no-op at runtime.
}
