# Contributing to `owl`

Thank you for your interest in Owl. Contributions are welcome.

Owl is currently being split out from Runme as a standalone Go module. The current code is a proof-of-concept baseline, and the near-term work is to clarify the package boundary before redesigning compound types, resolvers, and registry concepts.

## Prerequisites

You will need:

- Go compatible with the version in [go.mod](/go.mod).
- `make`.

## Setup

Download module dependencies:

```sh {"id":"01JZTPH8K7J7QJ4VC6Q2N6KWFA","name":"setup"}
go mod download
make install/dev
```

## Build

Build the `owl` CLI binary:

```sh {"id":"01JZTPH8K7J7QJ4VC6Q3NYJ8BA","name":"build"}
make build
```

The binary is written to `./owl` by default.

## Test

Run the test suite:

```sh {"id":"01JZTPH8K7J7QJ4VC6Q4KW6P0B","name":"test","terminalRows":"15"}
make test
```

## Format

Format Go files:

```sh {"id":"01JZTPH8K7J7QJ4VC6Q5D4Z9AP","name":"fmt"}
make fmt
```

## Lint

Run the current lint target:

```sh {"id":"01JZTPH8K7J7QJ4VC6Q6HF3791","name":"lint","terminalRows":"15"}
make lint
```

## Check

Run the local development gate:

```sh {"id":"01JZTPH8K7J7QJ4VC6Q7C3N6VJ","name":"check","terminalRows":"15"}
make check
```
