#### Building

Ensure your version of go is up-to-date, check that you're running at least 1.16 with the command:
```sh
$ go version
```

To build the go binary, run:
```sh
$ make build
```

which will create a binary for your current platform in the repository root folder called `must-gather-clean`.

## Testing

This project uses the built-in testing support for golang.

To run the tests for all go packages, run:
```sh
$ make test
```

## Dependency Management

## Formatting Code

To automatically format the code to conform to our style guide, you can run:
```sh
$ make update-gofmt
```

## Regenerate the configuration schema

The schema is defined in JSONSCHEMA and stored in `pkg/schema/schema.json`. 

All changes to the schema and its documentation must go through here, only then generate the go code from it using:
```sh
$ make update-scripts
```

There might be cases where the generation fails or creates suboptimal checks or missing validations. Please watch out for the generated code as well.

In case we need to adapt or fix anything, we have the [`go-jsonschema` repository forked](https://github.com/tjungblu/go-jsonschema) with a couple of patches already.

## Linting and schema verification

We're using a [`golangci-lint`](https://github.com/golangci/golangci-lint) as our linter in addition to `go vet`. You can run: 
```sh
$ make verify
```

to make sure the schema is up-to-date and all the code conforms to our style.
