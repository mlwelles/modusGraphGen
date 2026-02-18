// modusGraphGen is a code generation tool that reads Go structs with dgraph
// struct tags and produces a typed client library, functional options, query
// builders, and a Kong CLI.
//
// Usage:
//
//	go run github.com/mlwelles/modusGraphGen [flags]
//
// When invoked via go:generate (the typical case), it uses the current working
// directory as the target package.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mlwelles/modusGraphGen/parser"
)

func main() {
	pkgDir := flag.String("pkg", ".", "path to the target Go package directory")
	flag.Parse()

	// Resolve the package directory.
	dir := *pkgDir
	if dir == "." {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			log.Fatalf("failed to get working directory: %v", err)
		}
	}

	// Parse phase: extract the model from Go source files.
	pkg, err := parser.Parse(dir)
	if err != nil {
		log.Fatalf("parse error: %v", err)
	}

	fmt.Printf("Package: %s\n", pkg.Name)
	fmt.Printf("Entities: %d\n", len(pkg.Entities))
	for _, e := range pkg.Entities {
		searchInfo := ""
		if e.Searchable {
			searchInfo = fmt.Sprintf(" (searchable on %s)", e.SearchField)
		}
		fmt.Printf("  - %s: %d fields%s\n", e.Name, len(e.Fields), searchInfo)
		for _, f := range e.Fields {
			extras := ""
			if f.IsUID {
				extras = " [UID]"
			} else if f.IsDType {
				extras = " [DType]"
			} else if f.IsEdge {
				rev := ""
				if f.IsReverse {
					rev = ", reverse"
				}
				cnt := ""
				if f.HasCount {
					cnt = ", count"
				}
				extras = fmt.Sprintf(" [edge -> %s%s%s]", f.EdgeEntity, rev, cnt)
			}
			if len(f.Indexes) > 0 {
				extras += fmt.Sprintf(" indexes=%v", f.Indexes)
			}
			if f.Upsert {
				extras += " upsert"
			}
			fmt.Printf("    %s (%s) predicate=%q%s\n", f.Name, f.GoType, f.Predicate, extras)
		}
	}

	// TODO: Generate phase - templates will be filled in later.
	fmt.Println("\n[codegen templates not yet implemented]")
}
