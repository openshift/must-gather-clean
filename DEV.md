## Regenerate the configuration schema

The schema is defined in JSONSCHEMA and stored in `pkg/schema/schema.json`. 

All changes to the schema and its documentation must go through here, only then generate the go code from it using:

> make update-scripts

There might be cases where the generation fails or creates suboptimal checks or missing validations. 
Please watch out for the generated code as well.
In case we need to adapt or fix anything, we have the [`go-jsonschema` repository forked](https://github.com/tjungblu/go-jsonschema) with a couple of patches already.

## Linting and schema verification

We're using a [`golangci-lint`](https://github.com/golangci/golangci-lint) as our linter in addition to `go vet`. 
You can run 

> make verify

to make sure the schema is up-to-date and all the code conforms to our style.