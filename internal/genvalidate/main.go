// Command genvalidate generates a per-service request-body validation table
// (validate_gen.go) from the service's OpenAPI spec. The generated file maps
// "METHOD /path" route keys to *core.BodySchema literals evaluated by
// core.BodyValidator; regenerating after a spec update surfaces validation
// changes as a reviewable Go diff.
//
// Spec handling: OpenAPI 3.0 and 3.1 are accepted (`nullable: true` and
// `type: ["T","null"]`), local $refs are resolved and allOf is merged.
// oneOf/anyOf/not, recursive refs, and non-RE2 patterns degrade the schema
// to permissive with a `// NOTE:` in the output. A required name with no
// property definition beside it (a spec inconsistency that ogen-generated SDK
// types also drop) is skipped with a NOTE. Spec paths the mock does not serve
// yield inert entries; /_sakumock/ routes have none. -spec may be repeated
// to feed several specs into one table (simplemq: queue + message).
//
// -mapping bridges spec paths to mock route paths when they differ; see
// Mapping for the prefix / pathRewrites / routes / skipPaths keys.
//
// -responses additionally emits responseSchemas (route key -> declared
// status -> schema) compiled with response semantics: required writeOnly
// properties are skipped instead of readOnly ones; a `default` or error-range
// (4XX/5XX) response is ignored (undeclared error statuses are tolerated by
// the validator anyway, and dropping the route for a ubiquitous default
// catch-all would disable validation of the declared statuses), while a
// success-range (2XX/3XX) response degrades the route to permissive.
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
	responses := flag.Bool("responses", false, "also generate the responseSchemas table (per-status response-body constraints)")
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

	src, err := Generate(specs, pkg, *varName, mapping, *responses, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "genvalidate: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, src, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "genvalidate: %v\n", err)
		os.Exit(1)
	}
}
