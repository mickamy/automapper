# automapper

A CLI tool that automatically generates struct-to-struct mapping code for Go.

**Concept**: "Auto-map same name & type, tag only the exceptions"

## Installation

```bash
go install github.com/mickamy/automapper/cmd/automapper
```

## Basic Usage

```bash
# One-way mapping (User -> UserPB)
automapper -from=model.User -to=pb.UserPB

# Bidirectional mapping (User <-> UserPB)
automapper -types=model.User:pb.UserPB

# Custom output directory (default: ./amgen)
automapper -from=model.User -to=pb.UserPB -output=./mappers

# Specify package containing custom converters
automapper -from=model.User -to=pb.UserPB -converter-pkg=./converters
```

## Usage with go:generate

```go
//go:generate automapper -types=model.User:pb.UserPB
```

## Generated Code

```go
// amgen/user.gen.go
package amgen

// Pointer version
func ToPbUserPBPtr(src *model.User) *pb.UserPB { ... }

// Value version
func ToPbUserPB(src model.User) pb.UserPB { ... }

// Reverse mapping (when using -types for bidirectional)
func FromPbUserPBPtr(src *pb.UserPB) *model.User { ... }
func FromPbUserPB(src pb.UserPB) model.User { ... }
```

### When Converters Return Errors

If any registered converter returns an error (e.g., `RegisterFromE`), the generated function also returns an error:

```go
// Generated when using error-returning converters
func FromPbUserPBPtr(src *pb.UserPB) (*model.User, error) {
    if src == nil {
        return nil, nil
    }
    birthDate, err := converters.FromDate(src.BirthDate)
    if err != nil {
        return nil, fmt.Errorf("BirthDate: %w", err)
    }
    return &model.User{
        ID:        src.Id,
        Name:      src.Name,
        BirthDate: birthDate,
    }, nil
}

func FromPbUserPB(src pb.UserPB) (model.User, error) {
    birthDate, err := converters.FromDate(src.BirthDate)
    if err != nil {
        return model.User{}, fmt.Errorf("BirthDate: %w", err)
    }
    return model.User{
        ID:        src.Id,
        Name:      src.Name,
        BirthDate: birthDate,
    }, nil
}
```

## Mapping Rules

| Priority | Rule                      | Description                   |
|----------|---------------------------|-------------------------------|
| 1        | `map:"-"`                 | Ignore field                  |
| 2        | `map:"TargetName"`        | Map to specified field name   |
| 3        | `map:",conv=name"`        | Use named converter           |
| 4        | Same name & type          | Direct copy                   |
| 5        | Same name, different type | Look for registered converter |
| 6        | Target only               | Ignore (zero value)           |
| 7        | Unexported                | Ignore                        |

## Tag Examples

```go
type User struct {
    ID        int64     `map:"UserId"`           // Rename to UserId
    Password  string    `map:"-"`                // Ignore
    CreatedAt time.Time `map:",conv=timeToUnix"` // Use named converter
}
```

## Custom Converters

### Type-based Converters

Converters are automatically matched by type pair.

```go
package converters

import (
	"strconv"
	"time"
	
	"github.com/mickamy/automapper"
)

func init() {
    // time.Time -> int64
    automapper.RegisterTo[time.Time, int64](TimeToUnix)

    // int64 -> time.Time
    automapper.RegisterFrom[int64, time.Time](UnixToTime)

    // Converter that returns error
    automapper.RegisterToE[string, int](strconv.Atoi)
}

func TimeToUnix(t time.Time) int64 {
    return t.Unix()
}

func UnixToTime(ts int64) time.Time {
    return time.Unix(ts, 0)
}
```

### Named Converters

Use when you need to explicitly specify a converter via tag.

```go
func init() {
    // Referenced by map:",conv=priceToInt"
    automapper.RegisterToNamed[string, int64]("priceToInt", PriceStringToCents)

    // Named converter that returns error
    automapper.RegisterFromNamedE[int64, string]("priceFromInt", CentsToPriceString)
}

func PriceStringToCents(s string) int64 {
    // "$19.99" -> 1999
    ...
}

func CentsToPriceString(cents int64) (string, error) {
    // 1999 -> "$19.99"
    ...
}
```

Usage:

```go
type Product struct {
    Price string `map:",conv=priceToInt"` // Use named converter
}
```

## Register API

| Function                                     | Description              |
|----------------------------------------------|--------------------------|
| `RegisterTo[A, B](fn func(A) B)`             | Register A → B converter |
| `RegisterFrom[A, B](fn func(A) B)`           | Register A → B converter |
| `RegisterToE[A, B](fn func(A) (B, error))`   | A → B with error         |
| `RegisterFromE[A, B](fn func(A) (B, error))` | A → B with error         |
| `RegisterToNamed[A, B](name, fn)`            | Named A → B              |
| `RegisterFromNamed[A, B](name, fn)`          | Named A → B              |
| `RegisterToNamedE[A, B](name, fn)`           | Named A → B with error   |
| `RegisterFromNamedE[A, B](name, fn)`         | Named A → B with error   |

## CLI Flags

| Flag             | Description                                   | Default   |
|------------------|-----------------------------------------------|-----------|
| `-from`          | Source type (e.g., `model.User`)              | -         |
| `-to`            | Target type (e.g., `pb.UserPB`)               | -         |
| `-types`         | Bidirectional type pair (e.g., `User:UserPB`) | -         |
| `-output`        | Output directory                              | `./amgen` |
| `-converter-pkg` | Package containing converters                 | -         |
| `-version`       | Show version                                  | -         |

## License

[MIT](./LICENSE)
