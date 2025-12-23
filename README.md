# env-parser

A lightweight Go library for parsing environment variables into structs.

## Installation

```bash
go get github.com/pedrobarbosak/go-env-validator
```

## Usage

```go
package main

import (
    "fmt"
    "time"
    envParser "github.com/pedrobarbosak/go-env-validator"
)

type Config struct {
    Host     string        `env:"HOST,required"`
    Port     int           `env:"PORT,default=8080"`
    Debug    bool          `env:"DEBUG,default=false"`
    Timeout  time.Duration `env:"TIMEOUT,default=30s"`
    Hosts    []string      `env:"HOSTS,separator=|"`
    Labels   map[string]string `env:"LABELS"`
}

func main() {
    var cfg Config

    // From environment variables
    if err := envParser.UnmarshalFromEnv(&cfg); err != nil {
        panic(err)
    }

    // From .env file (merged with system env vars)
    if err := envParser.UnmarshalFromFile(".env", &cfg); err != nil {
        panic(err)
    }

    // From .env file only (ignores system env vars)
    if err := envParser.UnmarshalFromFileOnly(".env", &cfg); err != nil {
        panic(err)
    }

    fmt.Printf("%+v\n", cfg)
}
```

## Tag Options

| Option | Description | Example |
|--------|-------------|---------|
| `required` | Field must be present | `env:"HOST,required"` |
| `default=X` | Default value if not set | `env:"PORT,default=8080"` |
| `separator=X` | Separator for slices/maps | `env:"HOSTS,separator=\|"` |

Use `\,` to escape commas in tag values: `env:"ITEMS,separator=\,"`

## Supported Types

- `string`, `bool`
- `int`, `int8`, `int16`, `int32`, `int64`
- `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- `float32`, `float64`
- `time.Duration`
- `[]T` (slices of supported types)
- `map[string]T` (maps with string keys)
- Pointers to any supported type
- Nested structs

## Examples

### Environment Variables
```bash
HOST=localhost
PORT=3000
DEBUG=true
TIMEOUT=1m
HOSTS=host1|host2|host3
LABELS=env:prod;region:us
```

### Slices
```go
type Config struct {
    Hosts []string `env:"HOSTS,separator=|"`  // HOSTS=a|b|c
    Ports []int    `env:"PORTS,separator=;"`  // PORTS=80;443
}
```

### Maps
```go
type Config struct {
    Labels map[string]string `env:"LABELS"`  // LABELS=key1:val1;key2:val2
    Counts map[string]int    `env:"COUNTS"`  // COUNTS=errors:10;warnings:5
}
```

### Nested Structs
```go
type Database struct {
    Host string `env:"DB_HOST,required"`
    Port int    `env:"DB_PORT,default=5432"`
}

type Config struct {
    Database Database
}
```

## Global Configuration

```go
envParser.Tag = "env"        // Struct tag name (default: "env")
envParser.Separator = ";"    // Default separator for slices/maps (default: ";")
```

## Validation (Optional)

You can integrate any struct validator by implementing the `Validator` interface:

```go
type Validator interface {
    Struct(v interface{}) error
}
```

### With go-playground/validator

```go
import (
    "github.com/go-playground/validator/v10"
    envParser "github.com/pedrobarbosak/go-env-validator"
)

func init() {
    envParser.SetValidator(validator.New())
}

type Config struct {
    Host  string `env:"HOST" validate:"required,hostname"`
    Port  int    `env:"PORT" validate:"min=1,max=65535"`
    Email string `env:"EMAIL" validate:"email"`
}
```

Validation runs automatically after unmarshaling when a validator is set.
