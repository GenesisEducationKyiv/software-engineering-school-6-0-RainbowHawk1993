package architecture_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const modulePrefix = "releasesapi/"

type layer string

const (
	layerDomain         layer = "domain"
	layerPorts          layer = "ports"
	layerApplication    layer = "application"
	layerInfrastructure layer = "infrastructure"
	layerShared         layer = "shared"
	layerPlatform       layer = "platform"
	layerTransport      layer = "transport"
	layerCmd            layer = "cmd"
	layerGenerated      layer = "generated"
)

type dependencyRule struct {
	from layer
	to   layer
}

var allowedDependencies = map[dependencyRule]struct{}{
	{layerPorts, layerDomain}:             {},
	{layerApplication, layerDomain}:       {},
	{layerApplication, layerPorts}:        {},
	{layerApplication, layerPlatform}:     {},
	{layerApplication, layerShared}:       {},
	{layerInfrastructure, layerDomain}:    {},
	{layerInfrastructure, layerPorts}:     {},
	{layerInfrastructure, layerPlatform}:  {},
	{layerInfrastructure, layerGenerated}: {},
	{layerShared, layerPlatform}:          {},
	{layerShared, layerDomain}:            {},
	{layerTransport, layerDomain}:         {},
	{layerTransport, layerPorts}:          {},
	{layerTransport, layerPlatform}:       {},
	{layerTransport, layerGenerated}:      {},
	{layerCmd, layerDomain}:               {},
	{layerCmd, layerPorts}:                {},
	{layerApplication, layerApplication}:  {},
	{layerCmd, layerApplication}:          {},
	{layerCmd, layerInfrastructure}:       {},
	{layerCmd, layerPlatform}:             {},
	{layerCmd, layerShared}:               {},
	{layerCmd, layerTransport}:            {},
	{layerCmd, layerGenerated}:            {},
}

var explicitExceptions = map[string]map[string]struct{}{
	"releasesapi/internal/platform/config": {
		"releasesapi/internal/modules/notification": {},
	},
}

func classifyPackage(importPath string) (layer, bool) {
	switch {
	case strings.HasPrefix(importPath, modulePrefix+"gen/"):
		return layerGenerated, true
	case strings.HasPrefix(importPath, modulePrefix+"cmd/"):
		return layerCmd, true
	case strings.HasSuffix(importPath, "/domain"):
		return layerDomain, true
	case strings.HasSuffix(importPath, "/ports"):
		return layerPorts, true
	case strings.HasSuffix(importPath, "/application"):
		return layerApplication, true
	case strings.HasSuffix(importPath, "/infrastructure"):
		return layerInfrastructure, true
	case importPath == modulePrefix+"internal/modules/github":
		return layerShared, true
	case strings.HasPrefix(importPath, modulePrefix+"internal/modules/notification"):
		return layerShared, true
	case strings.HasPrefix(importPath, modulePrefix+"internal/platform/"):
		return layerPlatform, true
	case strings.HasPrefix(importPath, modulePrefix+"internal/transport/"):
		return layerTransport, true
	default:
		return "", false
	}
}

func TestLayerDependencies(t *testing.T) {
	t.Helper()

	root, err := findModuleRoot()
	if err != nil {
		t.Fatalf("find module root: %v", err)
	}

	violations, err := collectViolations(root)
	if err != nil {
		t.Fatalf("collect violations: %v", err)
	}

	if len(violations) == 0 {
		return
	}

	var report strings.Builder
	report.WriteString("architecture dependency violations:\n")
	for _, violation := range violations {
		fmt.Fprintf(&report, "  %s (%s) must not import %s (%s)\n",
			violation.fromPackage, violation.fromLayer,
			violation.toPackage, violation.toLayer,
		)
	}
	t.Fatal(report.String())
}

type violation struct {
	fromPackage string
	fromLayer   layer
	toPackage   string
	toLayer     layer
}

func collectViolations(root string) ([]violation, error) {
	var violations []violation

	for _, scanRoot := range []string{"internal", "cmd"} {
		scanPath := filepath.Join(root, scanRoot)
		if _, err := os.Stat(scanPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		err := filepath.WalkDir(scanPath, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			fromPackage, imports, err := packageImports(path, root)
			if err != nil {
				return err
			}

			fromLayer, ok := classifyPackage(fromPackage)
			if !ok {
				return nil
			}

			for _, importPath := range imports {
				if !strings.HasPrefix(importPath, modulePrefix) {
					continue
				}

				if isExplicitException(fromPackage, importPath) {
					continue
				}

				toLayer, ok := classifyPackage(importPath)
				if !ok {
					continue
				}

				if fromLayer == layerDomain && toLayer != layerGenerated {
					violations = append(violations, violation{
						fromPackage: fromPackage,
						fromLayer:   fromLayer,
						toPackage:   importPath,
						toLayer:     toLayer,
					})
					continue
				}

				if _, allowed := allowedDependencies[dependencyRule{from: fromLayer, to: toLayer}]; !allowed {
					violations = append(violations, violation{
						fromPackage: fromPackage,
						fromLayer:   fromLayer,
						toPackage:   importPath,
						toLayer:     toLayer,
					})
				}
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return violations, nil
}

func isExplicitException(fromPackage, toPackage string) bool {
	allowedImports, ok := explicitExceptions[fromPackage]
	if !ok {
		return false
	}

	_, allowed := allowedImports[toPackage]
	return allowed
}

func packageImports(path, moduleRoot string) (string, []string, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
	if err != nil {
		return "", nil, fmt.Errorf("parse %s: %w", path, err)
	}

	relDir, err := filepath.Rel(moduleRoot, filepath.Dir(path))
	if err != nil {
		return "", nil, err
	}

	packagePath := modulePrefix + filepath.ToSlash(relDir)
	imports := make([]string, 0, len(file.Imports))
	for _, imp := range file.Imports {
		imports = append(imports, strings.Trim(imp.Path.Value, `"`))
	}

	return packagePath, imports, nil
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}
