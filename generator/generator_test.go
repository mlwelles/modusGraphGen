package generator

import (
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mlwelles/modusGraphGen/parser"
)

var update = flag.Bool("update", false, "update golden files")

// moviesDir returns the absolute path to the movies package in the sibling
// modusGraphMoviesProject repository.
func moviesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// thisFile = .../modusGraphGen/generator/generator_test.go
	repoRoot := filepath.Dir(filepath.Dir(thisFile))
	return filepath.Join(filepath.Dir(repoRoot), "modusGraphMoviesProject", "movies")
}

// goldenDir returns the path to the golden test data directory.
func goldenDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "testdata", "golden")
}

func TestGenerate(t *testing.T) {
	dir := moviesDir(t)
	pkg, err := parser.Parse(dir)
	if err != nil {
		t.Fatalf("Parse(%s) failed: %v", dir, err)
	}

	// Generate to a temp directory.
	tmpDir := t.TempDir()
	if err := Generate(pkg, tmpDir); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	golden := goldenDir(t)

	if *update {
		// Copy all generated files to golden directory.
		t.Log("Updating golden files...")
		entries, err := os.ReadDir(tmpDir)
		if err != nil {
			t.Fatal(err)
		}
		// Clean golden dir first.
		_ = os.RemoveAll(golden)
		if err := os.MkdirAll(golden, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue // skip cmd/ directory for golden tests
			}
			src := filepath.Join(tmpDir, entry.Name())
			dst := filepath.Join(golden, entry.Name())
			data, err := os.ReadFile(src)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dst, data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		t.Log("Golden files updated.")
		return
	}

	// Compare generated files against golden files.
	goldenEntries, err := os.ReadDir(golden)
	if err != nil {
		t.Fatalf("Reading golden dir %s: %v\nRun with -update to create golden files.", golden, err)
	}

	if len(goldenEntries) == 0 {
		t.Fatalf("No golden files found in %s. Run with -update to create them.", golden)
	}

	for _, entry := range goldenEntries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			goldenPath := filepath.Join(golden, name)
			generatedPath := filepath.Join(tmpDir, name)

			goldenData, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("reading golden file: %v", err)
			}

			generatedData, err := os.ReadFile(generatedPath)
			if err != nil {
				t.Fatalf("reading generated file: %v", err)
			}

			if string(goldenData) != string(generatedData) {
				t.Errorf("generated output differs from golden file %s", name)
				// Show a diff summary.
				goldenLines := strings.Split(string(goldenData), "\n")
				generatedLines := strings.Split(string(generatedData), "\n")
				maxLines := len(goldenLines)
				if len(generatedLines) > maxLines {
					maxLines = len(generatedLines)
				}
				diffCount := 0
				for i := 0; i < maxLines; i++ {
					var gl, genl string
					if i < len(goldenLines) {
						gl = goldenLines[i]
					}
					if i < len(generatedLines) {
						genl = generatedLines[i]
					}
					if gl != genl {
						if diffCount < 10 {
							t.Errorf("  line %d:\n    golden:    %q\n    generated: %q", i+1, gl, genl)
						}
						diffCount++
					}
				}
				if diffCount > 10 {
					t.Errorf("  ... and %d more differences", diffCount-10)
				}
			}
		})
	}
}

func TestGenerateOutputFiles(t *testing.T) {
	dir := moviesDir(t)
	pkg, err := parser.Parse(dir)
	if err != nil {
		t.Fatalf("Parse(%s) failed: %v", dir, err)
	}

	tmpDir := t.TempDir()
	if err := Generate(pkg, tmpDir); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify expected files were created.
	expectedFiles := []string{
		"client_gen.go",
		"page_options_gen.go",
		"iter_gen.go",
	}

	// Per-entity files.
	entities := []string{
		"actor", "content_rating", "country", "director",
		"film", "genre", "location", "performance", "rating",
	}
	for _, e := range entities {
		expectedFiles = append(expectedFiles,
			e+"_gen.go",
			e+"_options_gen.go",
			e+"_query_gen.go",
		)
	}

	for _, f := range expectedFiles {
		t.Run(f, func(t *testing.T) {
			path := filepath.Join(tmpDir, f)
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("expected file %s not found: %v", f, err)
			}
			if info.Size() == 0 {
				t.Errorf("file %s is empty", f)
			}
		})
	}

	// Verify CLI stub.
	cliPath := filepath.Join(tmpDir, "cmd", "movies", "main.go")
	if _, err := os.Stat(cliPath); err != nil {
		t.Errorf("CLI stub not found: %v", err)
	}
}

func TestGenerateHeader(t *testing.T) {
	dir := moviesDir(t)
	pkg, err := parser.Parse(dir)
	if err != nil {
		t.Fatalf("Parse(%s) failed: %v", dir, err)
	}

	tmpDir := t.TempDir()
	if err := Generate(pkg, tmpDir); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check that all generated files start with the expected header.
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(tmpDir, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(data), "// Code generated by modusGraphGen. DO NOT EDIT.") {
				t.Errorf("file %s does not start with expected header", entry.Name())
			}
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Film", "film"},
		{"ContentRating", "content_rating"},
		{"UID", "uid"},
		{"HTTPServer", "http_server"},
		{"Actor", "actor"},
		{"Performance", "performance"},
		{"Location", "location"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toSnakeCase(tt.input)
			if got != tt.want {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSearchPredicate(t *testing.T) {
	dir := moviesDir(t)
	pkg, err := parser.Parse(dir)
	if err != nil {
		t.Fatalf("Parse(%s) failed: %v", dir, err)
	}

	for _, entity := range pkg.Entities {
		if entity.Searchable {
			pred := searchPredicate(entity)
			if pred == "" {
				t.Errorf("entity %s is searchable but searchPredicate returned empty", entity.Name)
			}
			t.Logf("%s: search predicate = %q", entity.Name, pred)
		}
	}
}
