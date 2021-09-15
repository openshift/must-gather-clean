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

## Manually releasing to GitHub

First tag the git commit that you want to release using the 'v' prefix and choose the next version you want to release, for example:
```sh 
$ git tag v0.0.1
$ git push --tags
```

then it's time to compile the binaries, you can do that with:
```sh 
$ make cross
```

There you should already see the tag being populated as the ld flag:
> ./hack/compile.sh -mod=vendor -ldflags="-s -w -X github.com/openshift/must-gather-clean/pkg/version.versionFromGit="v0.0.1-g8d647ad" -X github.com/openshift/must-gather-clean/pkg/version.commitFromGit="8d647ad" -X github.com/openshift/must-gather-clean/pkg/version.gitTreeState="clean" -X github.com/openshift/must-gather-clean/pkg/version.buildDate="2021-09-15T08:30:58Z""

Double-check the binary was created with the right version:
```sh 
$ dist/bin/linux-amd64/must-gather-clean version
Version: pkg.Version{Version:"v0.0.1-g8d647ad", GitCommit:"8d647ad", BuildDate:"2021-09-15T08:30:58Z", GoOs:"linux", GoArch:"amd64"}
```

Then you can run the prep release target and you should see the binaries and checksums popping up:

```sh
$ make prepare-release

$ ls -l dist/release/
total 29428
-rwxrwxr-x. 1 tjungblu tjungblu 4572912 15. Sep 10:33 must-gather-clean-darwin-amd64
-rw-rw-r--. 1 tjungblu tjungblu 1724044 15. Sep 10:33 must-gather-clean-darwin-amd64.tar.gz
-rwxrwxr-x. 1 tjungblu tjungblu 4534434 15. Sep 10:33 must-gather-clean-darwin-arm64
-rw-rw-r--. 1 tjungblu tjungblu 1671871 15. Sep 10:33 must-gather-clean-darwin-arm64.tar.gz
-rwxrwxr-x. 1 tjungblu tjungblu 4251648 15. Sep 10:33 must-gather-clean-linux-amd64
-rw-rw-r--. 1 tjungblu tjungblu 1655452 15. Sep 10:33 must-gather-clean-linux-amd64.tar.gz
-rwxrwxr-x. 1 tjungblu tjungblu 4063232 15. Sep 10:33 must-gather-clean-linux-arm64
-rw-rw-r--. 1 tjungblu tjungblu 1489029 15. Sep 10:33 must-gather-clean-linux-arm64.tar.gz
-rwxrwxr-x. 1 tjungblu tjungblu 4453888 15. Sep 10:33 must-gather-clean-windows-amd64.exe
-rw-rw-r--. 1 tjungblu tjungblu 1692325 15. Sep 10:33 must-gather-clean-windows-amd64.exe.zip
-rw-rw-r--. 1 tjungblu tjungblu     998 15. Sep 10:33 SHA256_SUM
```

You can then go to [GitHub Releases](https://github.com/openshift/must-gather-clean/releases/new) and create a new release from that tag. 

You can attach all the binaries (archives are not that useful, but you can include them) from the dist/release folder and upload them. Hit the release button and don't forget to :party-doge:.