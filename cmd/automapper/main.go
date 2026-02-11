// Package main provides the automapper CLI.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/mickamy/automapper/internal/analyzer"
	"github.com/mickamy/automapper/internal/generator"
	"github.com/mickamy/automapper/internal/registry"
	"github.com/mickamy/automapper/internal/resolver"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "automapper: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Define flags
	var (
		fromType     = flag.String("from", "", "Source type (e.g., User)")
		toType       = flag.String("to", "", "Target type (e.g., userpb.User)")
		types        = flag.String("types", "", "Bidirectional type pair (e.g., User:userpb.User)")
		output       = flag.String("output", "./amgen", "Output directory")
		converterPkg = flag.String("converter-pkg", "", "Package containing converter registrations")
		showVersion  = flag.Bool("version", false, "Show version")
	)

	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("automapper version %s\n", version)

		return nil
	}

	// Also handle -v and -V for version
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "-V" {
			fmt.Printf("automapper version %s\n", version)

			return nil
		}
	}

	// Parse type specifications
	var mappings []typePair

	switch {
	case *types != "":
		// Bidirectional: User:userpb.User
		parts := strings.SplitN(*types, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid -types format, expected A:B")
		}
		mappings = append(mappings,
			typePair{From: parts[0], To: parts[1], Bidirectional: true},
		)
	case *fromType != "" && *toType != "":
		mappings = append(mappings,
			typePair{From: *fromType, To: *toType, Bidirectional: false},
		)
	default:
		flag.Usage()

		return fmt.Errorf("must specify either -from/-to or -types")
	}

	// Load current package to resolve types
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	a := analyzer.New()
	currentPkg, err := a.LoadPackage(cwd)
	if err != nil {
		return fmt.Errorf("load current package: %w", err)
	}

	// Load converter registrations
	reg := registry.New()
	if *converterPkg != "" {
		convPkg, err := a.LoadPackage(*converterPkg)
		if err != nil {
			return fmt.Errorf("load converter package: %w", err)
		}
		converters, err := analyzer.DiscoverConverters(convPkg)
		if err != nil {
			return fmt.Errorf("discover converters: %w", err)
		}
		reg.LoadFromConverterInfos(converters, convPkg)
	}

	// Also scan current package for converters
	converters, err := analyzer.DiscoverConverters(currentPkg)
	if err != nil {
		return fmt.Errorf("discover converters in current package: %w", err)
	}
	reg.LoadFromConverterInfos(converters, currentPkg)

	// Resolve output directory
	outputDir := *output
	if !filepath.IsAbs(outputDir) {
		outputDir = filepath.Join(cwd, outputDir)
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Determine output package name
	outputPkgName := filepath.Base(outputDir)
	if outputDir == cwd {
		outputPkgName = currentPkg.Name
	}

	// Process each mapping
	for _, mp := range mappings {
		if err := processMapping(a, reg, currentPkg, mp, outputDir, outputPkgName); err != nil {
			return err
		}
	}

	return nil
}

type typePair struct {
	From          string
	To            string
	Bidirectional bool
}

func processMapping(a *analyzer.Analyzer, reg *registry.Registry, currentPkg *packages.Package, mp typePair, outputDir, outputPkgName string) error {
	// Resolve source type
	sourceInfo, err := resolveType(a, currentPkg, mp.From)
	if err != nil {
		return fmt.Errorf("resolve source type %s: %w", mp.From, err)
	}

	// Resolve target type
	targetInfo, err := resolveType(a, currentPkg, mp.To)
	if err != nil {
		return fmt.Errorf("resolve target type %s: %w", mp.To, err)
	}

	// Create resolver
	res := resolver.New(reg)

	// Collect all mappings including nested structs
	var forwardMappings []*resolver.Mapping
	var reverseMappings []*resolver.Mapping

	// Queue of type pairs to process
	type structPair struct {
		source *analyzer.StructInfo
		target *analyzer.StructInfo
	}
	queue := []structPair{{source: sourceInfo, target: targetInfo}}
	processed := make(map[string]bool)

	for len(queue) > 0 {
		pair := queue[0]
		queue = queue[1:]

		key := pair.source.PkgPath + "." + pair.source.Name + "->" + pair.target.PkgPath + "." + pair.target.Name
		if processed[key] {
			continue
		}
		processed[key] = true

		// Resolve forward mapping
		forwardMapping, errs := res.Resolve(pair.source, pair.target)
		if errs.HasErrors() {
			return fmt.Errorf("resolve forward mapping %s -> %s: %w", pair.source.Name, pair.target.Name, errs)
		}
		forwardMappings = append(forwardMappings, forwardMapping)

		// Resolve reverse mapping if bidirectional
		if mp.Bidirectional {
			reverseMapping, errs := res.Resolve(pair.target, pair.source)
			if errs.HasErrors() {
				return fmt.Errorf("resolve reverse mapping %s -> %s: %w", pair.target.Name, pair.source.Name, errs)
			}
			reverseMappings = append(reverseMappings, reverseMapping)
		}

		// Find nested structs that need their own mappers
		for _, fm := range forwardMapping.Fields {
			if fm.Kind == resolver.KindNested || fm.Kind == resolver.KindSlice {
				// Get the nested struct types
				sourceNested := analyzer.Dereference(fm.SourceType)
				targetNested := analyzer.Dereference(fm.TargetType)

				if fm.Kind == resolver.KindSlice {
					sourceNested = analyzer.SliceElem(fm.SourceType)
					targetNested = analyzer.SliceElem(fm.TargetType)
					if sourceNested != nil {
						sourceNested = analyzer.Dereference(sourceNested)
					}
					if targetNested != nil {
						targetNested = analyzer.Dereference(targetNested)
					}
				}

				if sourceNested == nil || targetNested == nil {
					continue
				}

				// Only process if both are named struct types
				if !analyzer.IsStruct(sourceNested) || !analyzer.IsStruct(targetNested) {
					continue
				}

				// Get struct info for nested types
				sourcePkg := analyzer.TypePkgPath(sourceNested)
				targetPkg := analyzer.TypePkgPath(targetNested)
				sourceName := analyzer.TypeName(sourceNested)
				targetName := analyzer.TypeName(targetNested)

				if sourcePkg == "" || targetPkg == "" {
					continue
				}

				sourceNestedPkg, err := a.LoadPackage(sourcePkg)
				if err != nil {
					continue
				}
				targetNestedPkg, err := a.LoadPackage(targetPkg)
				if err != nil {
					continue
				}

				sourceNestedInfo, err := a.FindStruct(sourceNestedPkg, sourceName)
				if err != nil {
					continue
				}
				targetNestedInfo, err := a.FindStruct(targetNestedPkg, targetName)
				if err != nil {
					continue
				}

				queue = append(queue, structPair{source: sourceNestedInfo, target: targetNestedInfo})
			}
		}
	}

	// Generate code
	gen := generator.New(reg, outputPkgName, outputDir)

	// Generate all forward mappings in one file
	forwardCode, err := gen.GenerateFile(forwardMappings, "to")
	if err != nil {
		return fmt.Errorf("generate forward code: %w", err)
	}

	// Determine output filename
	outputFile := strings.ToLower(sourceInfo.Name) + ".gen.go"
	if outputDir == "." {
		outputFile = strings.ToLower(sourceInfo.Name) + "_" + strings.ToLower(strings.ReplaceAll(targetInfo.QualifiedName(), ".", "_")) + ".gen.go"
	}

	outputPath := filepath.Join(outputDir, outputFile)
	if err := os.WriteFile(outputPath, forwardCode, 0600); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}

	fmt.Printf("Generated %s\n", outputPath)

	// Generate reverse if bidirectional
	if mp.Bidirectional && len(reverseMappings) > 0 {
		reverseCode, err := gen.GenerateFile(reverseMappings, "from")
		if err != nil {
			return fmt.Errorf("generate reverse code: %w", err)
		}

		reverseFile := strings.ToLower(targetInfo.Name) + "_reverse.gen.go"
		reversePath := filepath.Join(outputDir, reverseFile)
		if err := os.WriteFile(reversePath, reverseCode, 0600); err != nil {
			return fmt.Errorf("write reverse output file: %w", err)
		}

		fmt.Printf("Generated %s\n", reversePath)
	}

	return nil
}

func resolveType(a *analyzer.Analyzer, currentPkg *packages.Package, typeStr string) (*analyzer.StructInfo, error) {
	// Check if type contains a dot (qualified name)
	if strings.Contains(typeStr, ".") {
		info, err := a.ResolveType(typeStr, currentPkg)
		if err != nil {
			return nil, fmt.Errorf("resolve type %s: %w", typeStr, err)
		}

		return info, nil
	}
	// Unqualified type - look in current package
	info, err := a.FindStruct(currentPkg, typeStr)
	if err != nil {
		return nil, fmt.Errorf("find struct %s: %w", typeStr, err)
	}

	return info, nil
}
