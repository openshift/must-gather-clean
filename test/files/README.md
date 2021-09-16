## e2e test suite

This directory contains archives of [must-gather](https://github.com/openshift/must-gather) outputs from these environments:
* aws
* azure
* gcp
* metal (ipv6)
* vsphere

Each archive has coresponding report (`-report.yaml`) that acts as the source of thruth for the e2e test.

Use `make test-e2e` to execute the test suite.
