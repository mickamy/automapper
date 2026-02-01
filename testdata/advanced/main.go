// Package advanced is a test package for automapper with tags and converters.
package advanced

import (
	// Register converters for automapper code generation.
	_ "github.com/mickamy/automapper/testdata/advanced/converters"

	"github.com/mickamy/automapper/testdata/advanced/model"
	"github.com/mickamy/automapper/testdata/advanced/pb"
)

//go:generate go run github.com/mickamy/automapper/cmd -types=model.Order:pb.OrderPB -converter-pkg=./converters

// Ensure types are used.
var (
	_ model.Order
	_ pb.OrderPB
)
