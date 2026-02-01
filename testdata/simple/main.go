// Package simple is a test package for automapper.
package simple

import (
	"github.com/mickamy/automapper/testdata/simple/model"
	"github.com/mickamy/automapper/testdata/simple/pb"
)

//go:generate go run github.com/mickamy/automapper/cmd -from=model.User -to=pb.UserPB

// Ensure types are used
var (
	_ model.User
	_ pb.UserPB
)
