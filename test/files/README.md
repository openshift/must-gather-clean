## e2e test suite

This directory contains archives of [must-gather](https://github.com/openshift/must-gather) outputs from these environments:
* aws
* azure
* azure-ci-job
* gcp
* metal (ipv6)
* vsphere

Each archive has corresponding report (`-report.yaml`) that acts as the source of truth for the e2e test.

The exact report comparison and report generation use one worker so that discovery
order and report contents are deterministic. A separate parallel-worker test runs
against the Azure CI job fixture with at least two workers. That test checks
worker-independent invariants instead of comparing reports exactly, because worker
scheduling can change opaque replacement assignment and Azure occurrence counts.

Use `make test-e2e` to execute the test suite.

### Update the reports

Sometimes the test configs or the implementation changes and the expected reports need to be regenerated.

On the repository root you can simply run:

```ssh
$ make gen-e2e
```

this will run the current build of `must-gather-clean` to generate reports and put them in the right place.
Please manually ensure that the diff is expected.
