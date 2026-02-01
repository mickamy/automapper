// Package witherror demonstrates error-returning converters.
package witherror

import (
	// Register converters for automapper code generation.
	_ "github.com/mickamy/automapper/testdata/witherror/converters"

	"github.com/mickamy/automapper/testdata/witherror/model"
	"github.com/mickamy/automapper/testdata/witherror/pb"
)

//go:generate go run github.com/mickamy/automapper/cmd -types=model.User:pb.UserPB -converter-pkg=./converters

// Ensure types are used.
var (
	_ model.User
	_ pb.UserPB
)
