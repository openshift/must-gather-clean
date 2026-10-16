## e2e test suite

This directory contains archives of [must-gather](https://github.com/openshift/must-gather) outputs from these environments:
* aws
* azure
* gcp
* metal (ipv6)
* vsphere

Each archive has corresponding report (`-report.yaml`) that acts as the source of truth for the e2e test.

Use `make test-e2e` to execute the test suite.

### Update the reports

Sometimes the test configs or the implementation changes and the expected reports need to be regenerated.

On the repository root you can simply run:

```ssh
$ make gen-e2e
```

this will run the current build of `must-gather-clean` to generate reports and put them in the right place.
Please manually ensure that the diff is expected.
