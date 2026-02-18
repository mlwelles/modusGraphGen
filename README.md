# modusGraphGen

Code generation tool for [modusgraph](https://github.com/matthewmcneely/modusgraph).
Reads Go structs with `json` and `dgraph` tags and produces a fully typed client
library, query builders, auto-paging iterators, and a
[Kong](https://github.com/alecthomas/kong) CLI. The generated code provides
type-safe CRUD operations, fulltext search, cursor-based pagination, and
fluent query building — all derived from your struct definitions.

modusGraphGen follows a **struct-first** approach: your Go structs are the
single source of truth. There are no separate schema files, no manual client
wiring, and no runtime reflection. The generator parses struct tags at build
time and emits concrete, readable Go code.

## Install

Add as a tool dependency in your `go.mod` (Go 1.24+):

```
tool github.com/mlwelles/modusGraphGen
```

Then run:

```sh
go mod tidy
```

For local development with a cloned copy, add a `replace` directive:

```
replace github.com/mlwelles/modusGraphGen => ../modusGraphGen
```

## Quick Start

### 1. Define your structs

Create Go structs with `json` and `dgraph` tags. Each struct that has both a
`UID` field and a `DType` field is recognized as a Dgraph entity and gets its
own typed sub-client:

```go
// movies/film.go
package movies

import "time"

type Film struct {
    UID                string    `json:"uid,omitempty"`
    DType              []string  `json:"dgraph.type,omitempty"`
    Name               string    `json:"name,omitempty" dgraph:"index=hash,term,trigram,fulltext"`
    InitialReleaseDate time.Time `json:"initialReleaseDate,omitempty" dgraph:"predicate=initial_release_date index=year"`
    Tagline            string    `json:"tagline,omitempty"`
    Genres             []Genre   `json:"genres,omitempty" dgraph:"predicate=genre reverse count"`
}

type Genre struct {
    UID   string   `json:"uid,omitempty"`
    DType []string `json:"dgraph.type,omitempty"`
    Name  string   `json:"name,omitempty" dgraph:"index=hash,term,trigram,fulltext"`
    Films []Film   `json:"films,omitempty" dgraph:"predicate=~genre reverse"`
}
```

### 2. Add a generate directive

```go
// movies/generate.go
package movies

//go:generate go run github.com/mlwelles/modusGraphGen
```

### 3. Run code generation

```sh
go generate ./movies
```

This produces typed client code and a Kong CLI in your package directory. All
generated files end in `_gen.go` so they're easy to identify and gitignore if
desired.

## Struct Tags

modusGraphGen reads two struct tag systems to understand your data model:

### The `json` Tag

Standard Go JSON serialization tag. modusGraphGen uses it to determine:
- The **default predicate name** (when no `predicate=` is specified in `dgraph`)
- Whether the field uses **omitempty** semantics

```go
Name string `json:"name,omitempty"`
//                 ^^^^             — predicate defaults to "name"
//                      ^^^^^^^^^^  — omit from mutations when zero value
```

### The `dgraph` Tag

Controls how the field maps to Dgraph's schema. Uses **space-separated**
directives. Commas appear only within `index=` to separate multiple index types.

```go
// Space separates independent directives:
dgraph:"predicate=initial_release_date index=year"

// Commas separate index tokenizers within index=:
dgraph:"index=hash,term,trigram,fulltext"

// Forward edge with reverse indexing and count:
dgraph:"predicate=genre reverse count"

// Reverse edge (BOTH ~ prefix AND reverse keyword required):
dgraph:"predicate=~genre reverse"

// Standalone flags:
dgraph:"count"
dgraph:"upsert"
```

### Tag Directives Reference

| Directive | Example | Effect |
|-----------|---------|--------|
| `predicate=X` | `predicate=initial_release_date` | Override the Dgraph predicate name. Default: json tag value |
| `predicate=~X` | `predicate=~genre` | Declare a reverse edge. Must also include `reverse` |
| `index=types` | `index=hash,term,trigram,fulltext` | Add search indexes (see Index Types below) |
| `reverse` | `reverse` | On forward edges: enables `~predicate` queries from the other side. On reverse edges (`predicate=~X`): **required** to set dgman's `ManagedReverse` flag so the edge is expanded in query results |
| `count` | `count` | Enable `count(predicate)` aggregate queries on this edge |
| `upsert` | `upsert` | Mark field for upsert deduplication (find-or-create by this value) |
| `type=X` | `type=geo` | Dgraph type hint for non-standard types (geo, password, etc.) |

### String Index Types

Dgraph offers several index types for `string` predicates. Specify one or more
with `index=` (comma-separated). Each enables different DQL filter functions:

| Index | DQL Functions | Description |
|-------|---------------|-------------|
| `hash` | `eq` | Fast equality check. Hashes the full string, so efficient for long values. Use when you only need exact match |
| `exact` | `eq`, `lt`, `le`, `gt`, `ge` | Stores the full string for equality and lexicographic comparison. Use when you need inequality filters on strings |
| `term` | `allofterms`, `anyofterms` | Splits the string into whitespace-delimited terms. `allofterms` matches when ALL terms are present; `anyofterms` matches when ANY term is present |
| `fulltext` | `alloftext`, `anyoftext` | Full-text search with stemming and stop-word removal. "run" matches "running" and "ran". Supports 18 languages. **This is the index that triggers `Search()` generation** |
| `trigram` | `regexp` | Decomposes the string into 3-character substrings (trigrams) for regular expression matching. Efficient when the regex contains long literal substrings |

**Combining indexes**: You can specify multiple index types on the same field.
For example, a `Name` field that needs fulltext search, exact match, term
matching, and regex support:

```go
Name string `json:"name,omitempty" dgraph:"index=hash,term,trigram,fulltext"`
```

### Scalar Index Types

Non-string types have their own index options:

| Go Type | Dgraph Type | Index | DQL Functions |
|---------|-------------|-------|---------------|
| `int`, `int64` | `int` | (default) | `eq`, `lt`, `le`, `gt`, `ge` |
| `float64` | `float` | (default) | `eq`, `lt`, `le`, `gt`, `ge` |
| `bool` | `bool` | (default) | `eq` |
| `time.Time` | `datetime` | `year`, `month`, `day`, `hour` | `eq`, `lt`, `le`, `gt`, `ge` at specified granularity |
| `[]float64` | `geo` | `geo` (+ `type=geo`) | `near`, `within`, `contains`, `intersects` |

For datetime fields, the index granularity controls the precision:
- `index=year` — filter by year (most common for date ranges)
- `index=month` — filter down to month
- `index=day` — filter down to day
- `index=hour` — filter down to hour

### When `predicate=` Is Needed

By default, the Dgraph predicate name is the `json` tag value. Use
`predicate=` when they differ:

| Scenario | `json` tag | `predicate=` | Why |
|----------|-----------|-------------|-----|
| Snake case predicate | `initialReleaseDate` | `initial_release_date` | Dgraph uses snake_case, API uses camelCase |
| Dot-prefixed predicate | `films` | `director.film` | Namespaced predicate in the dataset |
| Singular vs plural | `genres` | `genre` | Dgraph predicate is singular, Go field is plural |
| Reverse edge | `films` | `~genre` | Traverse the `genre` edge backward |

### Forward vs Reverse Edges

**Forward edge** — Film points to Genre via the `genre` predicate:

```go
// Film.go
Genres []Genre `json:"genres,omitempty" dgraph:"predicate=genre reverse count"`
//                                              ^^^^^^^^^^^^^^^^ predicate name
//                                                               ^^^^^^^ allows ~genre queries
//                                                                       ^^^^^ enables count()
```

**Reverse edge** — Genre discovers which Films point to it:

```go
// Genre.go
Films []Film `json:"films,omitempty" dgraph:"predicate=~genre reverse"`
//                                           ^^^^^^^^^^^^^^^ ~ = reverse direction
//                                                           ^^^^^^^ REQUIRED for ManagedReverse
```

The `reverse` keyword is required on **both** sides:
- On the forward edge, it tells Dgraph to maintain a reverse index
- On the reverse edge, it tells dgman to set `ManagedReverse`, which causes
  the reverse edge to be expanded when querying

### Complete Struct Example

Here is a comprehensive example showing all tag features:

```go
package movies

import "time"

// Film is a Dgraph entity — has UID + DType fields.
type Film struct {
    // Required entity fields
    UID   string   `json:"uid,omitempty"`
    DType []string `json:"dgraph.type,omitempty"`

    // Scalar with multiple string indexes (triggers Search generation via fulltext)
    Name string `json:"name,omitempty" dgraph:"index=hash,term,trigram,fulltext"`

    // Datetime with custom predicate name and year-granularity index
    InitialReleaseDate time.Time `json:"initialReleaseDate,omitempty" dgraph:"predicate=initial_release_date index=year"`

    // Plain scalar (no dgraph tag needed — predicate defaults to json tag "tagline")
    Tagline string `json:"tagline,omitempty"`

    // Forward edge with reverse indexing and count
    Genres []Genre `json:"genres,omitempty" dgraph:"predicate=genre reverse count"`

    // Forward edge with reverse indexing (no count)
    Countries []Country `json:"countries,omitempty" dgraph:"predicate=country reverse"`
}

// Genre is a Dgraph entity with a reverse edge back to Film.
type Genre struct {
    UID   string   `json:"uid,omitempty"`
    DType []string `json:"dgraph.type,omitempty"`
    Name  string   `json:"name,omitempty" dgraph:"index=hash,term,trigram,fulltext"`

    // Reverse edge — lists Films that have this Genre
    Films []Film `json:"films,omitempty" dgraph:"predicate=~genre reverse"`
}

// Director uses a dot-prefixed predicate for its Films edge.
type Director struct {
    UID   string   `json:"uid,omitempty"`
    DType []string `json:"dgraph.type,omitempty"`
    Name  string   `json:"name,omitempty" dgraph:"index=hash,term,trigram,fulltext"`
    Films []Film   `json:"films,omitempty" dgraph:"predicate=director.film reverse count"`
}

// Location demonstrates geo and upsert features.
type Location struct {
    UID   string    `json:"uid,omitempty"`
    DType []string  `json:"dgraph.type,omitempty"`
    Name  string    `json:"name,omitempty" dgraph:"index=hash,term,trigram,fulltext"`
    Loc   []float64 `json:"loc,omitempty" dgraph:"index=geo type=geo"`
    Email string    `json:"email,omitempty" dgraph:"index=exact upsert"`
}
```

## Entity Detection

A struct is recognized as a Dgraph entity when it has **both** of these fields:

```go
UID   string   `json:"uid,omitempty"`          // identifies the node in Dgraph
DType []string `json:"dgraph.type,omitempty"`  // Dgraph type discriminator
```

Structs without both fields are silently ignored by the generator. This lets
you define helper structs or value types in the same package without them being
treated as entities.

## What Gets Generated

For a package with N entity structs, modusGraphGen produces:

| File | Contents |
|------|----------|
| `client_gen.go` | `Client` struct with a sub-client field per entity, `New(connStr, opts...)`, `NewFromClient(conn)`, `Close()` |
| `page_options_gen.go` | `PageOption` interface, `First(n)`, `Offset(n)` — shared pagination across all entities |
| `iter_gen.go` | `SearchIter` (for entities with fulltext) and `ListIter` (for all entities) — auto-paging iterators using Go 1.23+ `iter.Seq2` |
| `<entity>_gen.go` | `<Entity>Client` struct with `Get`, `Add`, `Update`, `Delete`, `Search` (if fulltext), `List` |
| `<entity>_options_gen.go` | Functional options per entity (reserved for future use) |
| `<entity>_query_gen.go` | `<Entity>Query` builder with `Filter`, `OrderAsc`, `OrderDesc`, `First`, `Offset`, `Exec`, `ExecAndCount` |
| `cmd/<pkg>/main.go` | Complete Kong CLI with subcommands per entity |

### Inference Rules

The generator uses struct tags to decide what to generate:

| Struct characteristic | What gets generated |
|-----------------------|--------------------|
| Has `UID` + `DType` fields | Recognized as entity — gets `<Entity>Client` sub-client |
| String field with `index=fulltext` | `Search(ctx, term, opts...)` method + `SearchIter` iterator |
| Field typed `[]OtherEntity` | Recognized as edge relationship (handled by modusgraph at runtime) |
| `predicate=~X` with `reverse` | Reverse edge — dgman expands this when querying |
| Every entity (unconditionally) | `Get`, `Add`, `Update`, `Delete`, `List`, `ListIter`, `Query` builder |

## Generated API

### Client Setup

```go
import (
    "github.com/matthewmcneely/modusgraph"
    "github.com/your-org/your-project/movies"
)

// Connect to Dgraph via gRPC
client, err := movies.New("dgraph://localhost:9080",
    modusgraph.WithAutoSchema(true),  // auto-create schema from struct tags
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

The `Client` struct exposes a typed sub-client for every entity:

```go
client.Film          // *FilmClient — Get, Add, Update, Delete, Search, List, Query
client.Director      // *DirectorClient
client.Genre         // *GenreClient
client.Actor         // *ActorClient
// ... one per entity
```

### CRUD Operations

Every entity sub-client has `Get`, `Add`, `Update`, and `Delete`:

```go
ctx := context.Background()

// Add — inserts a new node, populates UID on the struct
film := &movies.Film{
    Name:               "The Matrix",
    InitialReleaseDate: time.Date(1999, 3, 31, 0, 0, 0, 0, time.UTC),
    Tagline:            "Welcome to the Real World",
    Genres:             []movies.Genre{action, scifi},  // edges set on insert
}
err := client.Film.Add(ctx, film)
fmt.Println(film.UID)  // "0x4e2a" — populated by Dgraph

// Get — retrieves a node by UID with edges expanded
got, err := client.Film.Get(ctx, "0x4e2a")
fmt.Println(got.Name)           // "The Matrix"
fmt.Println(len(got.Genres))    // 2 — edges are populated

// Update — modifies the node in place
got.Tagline = "There is no spoon"
err = client.Film.Update(ctx, got)

// Delete — removes the node by UID
err = client.Film.Delete(ctx, "0x4e2a")
```

### Fulltext Search

Generated for entities that have a string field with `index=fulltext`. Uses
Dgraph's `alloftext` function which supports stemming ("run" matches
"running", "ran") and stop-word removal:

```go
// Basic search
films, err := client.Film.Search(ctx, "Matrix")

// With pagination
films, err = client.Film.Search(ctx, "Matrix",
    movies.First(10),    // limit to 10 results
    movies.Offset(20),   // skip the first 20
)

// Search is generated per-entity — any entity with a fulltext field gets it
directors, err := client.Director.Search(ctx, "Coppola")
actors, err := client.Actor.Search(ctx, "Keanu")
genres, err := client.Genre.Search(ctx, "Action")
```

### List with Pagination

Retrieve entities with cursor-based pagination:

```go
page1, err := client.Film.List(ctx, movies.First(10))
page2, err := client.Film.List(ctx, movies.First(10), movies.Offset(10))

// List all genres
genres, err := client.Genre.List(ctx, movies.First(100))
```

### Query Builder

For complex queries combining filters, ordering, and pagination. The query
builder constructs DQL under the hood:

```go
var results []movies.Film

// Filter + order + limit
err := client.Film.Query(ctx).
    Filter(`alloftext(name, "Star")`).
    OrderAsc("name").
    First(5).
    Exec(&results)

// Order by date descending
err = client.Film.Query(ctx).
    First(10).
    OrderDesc("initial_release_date").
    Exec(&results)

// Count total matching results
count, err := client.Film.Query(ctx).
    Filter(`alloftext(name, "Matrix")`).
    First(10).
    ExecAndCount(&results)
fmt.Printf("Got %d results out of %d total\n", len(results), count)
```

**Common DQL filter patterns** for the `Filter` method:

```go
// Fulltext search (requires index=fulltext)
Filter(`alloftext(name, "Star Wars")`)

// Term matching (requires index=term)
Filter(`allofterms(name, "Star Wars")`)   // both "Star" AND "Wars" present
Filter(`anyofterms(name, "Star Wars")`)   // "Star" OR "Wars" present

// Equality (requires index=hash or index=exact)
Filter(`eq(name, "The Matrix")`)

// String inequality (requires index=exact)
Filter(`ge(name, "A") AND le(name, "M")`)

// Date range (requires index=year on datetime field)
Filter(`ge(initial_release_date, "1999-01-01") AND le(initial_release_date, "1999-12-31")`)

// Regular expression (requires index=trigram)
Filter(`regexp(name, /matrix/i)`)
```

### Auto-Paging Iterators

Uses Go 1.23+ `range`-over-func (`iter.Seq2`) to iterate through all pages
automatically. Each iteration fetches the next page of 50 results transparently:

```go
// Iterate over all films matching "Star Wars" (auto-pages in batches of 50)
for film, err := range client.Film.SearchIter(ctx, "Star Wars") {
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(film.Name)
}

// Iterate over all genres
for genre, err := range client.Genre.ListIter(ctx) {
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(genre.Name)
}

// Early termination works — just break out of the loop
for film, err := range client.Film.ListIter(ctx) {
    if err != nil { break }
    if film.Name == "The Matrix" {
        fmt.Println("Found it!")
        break  // stops fetching more pages
    }
}
```

`SearchIter` is generated only for entities with a fulltext-indexed field.
`ListIter` is generated for every entity.

### Generated CLI

The generated Kong CLI provides subcommands for every entity. Output is JSON
for easy piping to `jq`:

```sh
# Build the CLI
go build -o bin/movies ./movies/cmd/movies

# Search (available for entities with fulltext index)
./bin/movies film search "Matrix" --first=5
./bin/movies director search "Coppola"
./bin/movies actor search "Keanu"

# Get by UID
./bin/movies film get 0x4e2a

# List with pagination
./bin/movies genre list --first=20
./bin/movies film list --first=10 --offset=30

# Add
./bin/movies film add --name="New Film" --tagline="A great film"
./bin/movies genre add --name="Musical"

# Delete
./bin/movies film delete 0x4e2a

# Pipe to jq
./bin/movies film search "Star Wars" | jq '.[].name'
```

The CLI connects to Dgraph at `dgraph://localhost:9080` by default. Override
with `--addr` or the `DGRAPH_ADDR` environment variable.

## Flags

```
modusGraphGen [flags]

  -pkg string
        path to the target Go package directory (default ".")
  -output string
        output directory (default: same as -pkg)
```

When invoked via `go:generate`, the working directory is the package directory,
so the defaults work without flags.

## How It Works

modusGraphGen operates in three phases:

1. **Parse** — Uses `go/ast` and `go/parser` to walk the AST of all `.go` files
   in the target package. Extracts struct names, field types, and `json`/`dgraph`
   tags. Builds an intermediate `model.Package` with `Entity` and `Field` types.

2. **Infer** — Applies inference rules to the parsed model: detects entities
   (UID + DType), identifies searchable fields (fulltext index), resolves edge
   relationships (slice of another entity), and marks reverse edges (~ prefix).

3. **Generate** — Executes Go `text/template` templates embedded in the binary
   via `embed.FS`. Each template receives the model and produces a `_gen.go`
   file. The CLI template additionally produces `cmd/<pkg>/main.go`.

## Development

```sh
make help          # show all targets
make build         # build the modusGraphGen binary
make test          # run tests (requires ../modusGraphMoviesProject)
make check         # go vet
make update-golden # regenerate golden test files after template changes
```

### Golden File Tests

The generator tests parse struct definitions from the sibling
`modusGraphMoviesProject` repository and compare generated output against
checked-in golden files in `generator/testdata/golden/`. This ensures template
changes don't introduce regressions.

Update golden files after changing templates:

```sh
make update-golden
```

Then review the diff to confirm the changes are intentional.

## Reference Project

[modusGraphMoviesProject](https://github.com/mlwelles/modusGraphMoviesProject)
is the reference consumer of modusGraphGen, demonstrating the full workflow
with 9 entity types, forward and reverse edges, fulltext search, integration
tests, and Dgraph's 1-million movie dataset.

## License

Apache 2.0. See [LICENSE](LICENSE).
