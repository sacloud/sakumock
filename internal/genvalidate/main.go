// Command genvalidate generates a per-service request-body validation table
// (validate_gen.go) from the service's OpenAPI spec. The generated file maps
// "METHOD /path" route keys to *core.BodySchema literals evaluated by
// core.BodyValidator; regenerating after a spec update surfaces validation
// changes as a reviewable Go diff.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type multiFlag []string

func (m *multiFlag) String() string { return fmt.Sprint([]string(*m)) }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func main() {
	var specs multiFlag
	flag.Var(&specs, "spec", "OpenAPI spec file (JSON or YAML); repeatable")
	out := flag.String("out", "validate_gen.go", "output Go file path")
	mappingPath := flag.String("mapping", "", "optional route-mapping JSON file")
	varName := flag.String("var", "bodySchemas", "name of the generated map variable")
	pkgName := flag.String("pkg", "", "package name (default: base name of the output directory)")
	flag.Parse()

	if len(specs) == 0 {
		fmt.Fprintln(os.Stderr, "genvalidate: at least one -spec is required")
		os.Exit(2)
	}
	pkg := *pkgName
	if pkg == "" {
		abs, err := filepath.Abs(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "genvalidate: %v\n", err)
			os.Exit(1)
		}
		pkg = filepath.Base(filepath.Dir(abs))
	}

	var mapping *Mapping
	if *mappingPath != "" {
		m, err := LoadMapping(*mappingPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "genvalidate: %v\n", err)
			os.Exit(1)
		}
		mapping = m
	}

	src, err := Generate(specs, pkg, *varName, mapping, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "genvalidate: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, src, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "genvalidate: %v\n", err)
		os.Exit(1)
	}
}
