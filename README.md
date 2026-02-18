# modusGraphGen

Code generation tool for [modusgraph](https://github.com/matthewmcneely/modusgraph).
Reads Go structs with `dgraph` tags and produces a typed client library, query
builders, auto-paging iterators, and a Kong CLI.

## Install

Add as a tool dependency in your `go.mod`:

```
tool github.com/mlwelles/modusGraphGen
```

Then run:

```sh
go mod tidy
```

## Quick Start

1. Define your structs with `json` and `dgraph` tags:

```go
// movies/film.go
package movies

import "time"

type Film struct {
    UID                string    `json:"uid,omitempty"`
    DType              []string  `json:"dgraph.type,omitempty"`
    Name               string    `json:"name,omitempty" dgraph:"index=hash,term,trigram,fulltext"`
    InitialReleaseDate time.Time `json:"initialReleaseDate,omitempty" dgraph:"predicate=initial_release_date index=year"`
    Genres             []Genre   `json:"genres,omitempty" dgraph:"predicate=genre reverse count"`
}

type Genre struct {
    UID   string   `json:"uid,omitempty"`
    DType []string `json:"dgraph.type,omitempty"`
    Name  string   `json:"name,omitempty" dgraph:"index=hash,term,trigram,fulltext"`
    Films []Film   `json:"films,omitempty" dgraph:"predicate=~genre reverse"`
}
```

2. Add a generate directive:

```go
// movies/generate.go
package movies

//go:generate go run github.com/mlwelles/modusGraphGen
```

3. Run code generation:

```sh
go generate ./movies
```

This produces typed client code and a Kong CLI in your package directory.

## What Gets Generated

For a package with N entity structs, modusGraphGen produces:

| File | Contents |
|------|----------|
| `client_gen.go` | `Client` struct with sub-clients per entity, `New()`, `Close()` |
| `page_options_gen.go` | `First(n)` and `Offset(n)` pagination options |
| `iter_gen.go` | `SearchIter` and `ListIter` auto-paging iterators per entity |
| `<entity>_gen.go` | `Get`, `Add`, `Update`, `Delete`, `Search`, `List` per entity |
| `<entity>_options_gen.go` | Functional options per entity |
| `<entity>_query_gen.go` | Typed query builder per entity |
| `cmd/<pkg>/main.go` | Complete Kong CLI with subcommands per entity |

## Entity Detection

A struct is recognized as an entity when it has both `UID` and `DType` fields:

```go
type Film struct {
    UID   string   `json:"uid,omitempty"`          // required
    DType []string `json:"dgraph.type,omitempty"`  // required
    Name  string   `json:"name,omitempty"`
}
```

Structs without both fields are ignored by the generator.

## Tag Format

The `dgraph` tag uses **space-separated** tokens. Commas within `index=` are
tokenizer separators.

```go
// Space separates independent directives
dgraph:"predicate=initial_release_date index=year"

// Commas separate index tokenizers
dgraph:"index=hash,term,trigram,fulltext"

// Forward edge with reverse index and count
dgraph:"predicate=genre reverse count"

// Reverse edge (requires both ~ prefix AND reverse keyword)
dgraph:"predicate=~genre reverse"

// Standalone flags
dgraph:"count"
dgraph:"upsert"
```

### Inference Rules

| Struct characteristic | What gets generated |
|-----------------------|--------------------|
| Has `UID` + `DType` fields | Entity: gets a typed sub-client |
| String field with `index=fulltext` | `Search(ctx, term, opts...)` method |
| Field typed `[]OtherEntity` | Edge relationship |
| `predicate=~X` with `reverse` | Reverse edge |
| Every entity | `Get`, `Add`, `Update`, `Delete`, `List` |

## Generated API Examples

### Client Setup

```go
client, err := movies.New("dgraph://localhost:9080",
    modusgraph.WithAutoSchema(true),
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

### CRUD Operations

```go
// Add
film := &movies.Film{
    Name:               "The Matrix",
    InitialReleaseDate: time.Date(1999, 3, 31, 0, 0, 0, 0, time.UTC),
}
err := client.Film.Add(ctx, film)
// film.UID is now set

// Get by UID
got, err := client.Film.Get(ctx, film.UID)

// Update
got.Name = "The Matrix (1999)"
err = client.Film.Update(ctx, got)

// Delete
err = client.Film.Delete(ctx, film.UID)
```

### Search

Generated for entities with a `index=fulltext` field:

```go
films, err := client.Film.Search(ctx, "Matrix")

// With pagination
films, err = client.Film.Search(ctx, "Matrix",
    movies.First(10),
    movies.Offset(20),
)
```

### List with Pagination

```go
page1, err := client.Film.List(ctx, movies.First(10))
page2, err := client.Film.List(ctx, movies.First(10), movies.Offset(10))
```

### Query Builder

For complex queries combining filters, ordering, and pagination:

```go
var results []movies.Film
err := client.Film.Query(ctx).
    Filter(`alloftext(name, "Star")`).
    OrderAsc("name").
    First(5).
    Exec(&results)

// With count
count, err := client.Film.Query(ctx).
    Filter(`alloftext(name, "Matrix")`).
    First(10).
    ExecAndCount(&results)
```

### Auto-Paging Iterators

Uses Go 1.23+ range-over-func to iterate through all pages automatically:

```go
for film, err := range client.Film.SearchIter(ctx, "Star Wars") {
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(film.Name)
}

for genre, err := range client.Genre.ListIter(ctx) {
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(genre.Name)
}
```

### Generated CLI

The generated Kong CLI provides subcommands for every entity:

```
movies film search <term> [--first=10] [--offset=0]
movies film get <uid>
movies film list [--first=10] [--offset=0]
movies film add --name=<name>
movies film delete <uid>
movies director search <term>
movies genre list
```

Build and run:

```sh
go build -o bin/movies ./movies/cmd/movies
./bin/movies --help
./bin/movies film search "Matrix" --first=5
./bin/movies genre list
```

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

## Development

```sh
make help       # show all targets
make build      # build the binary
make test       # run tests (requires ../modusGraphMoviesProject)
make check      # go vet
```

### Golden File Tests

The generator tests parse struct definitions from the sibling
`modusGraphMoviesProject` repository and compare generated output against
checked-in golden files.

Update golden files after changing templates:

```sh
make update-golden
```

## Reference Project

[modusGraphMoviesProject](https://github.com/mlwelles/modusGraphMoviesProject)
is the reference consumer of modusGraphGen, demonstrating the full workflow
against Dgraph's 1-million movie dataset.

## License

Apache 2.0. See [LICENSE](LICENSE).
