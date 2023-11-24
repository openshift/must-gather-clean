module github.com/openshift/must-gather-clean

go 1.21

require (
	github.com/atombender/go-jsonschema v0.0.0-00010101000000-000000000000
	github.com/gijsbers/go-pcre v0.0.0-20161214203829-a84f3096ab3c
	github.com/openshift/build-machinery-go v0.0.0-20230824093055-6a18da01283c
	github.com/spf13/cobra v1.8.0
	github.com/stretchr/testify v1.8.4
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/klog/v2 v2.110.1
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/pkg/errors v0.8.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sanity-io/litter v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	gopkg.in/yaml.v2 v2.2.2 // indirect
)

replace github.com/atombender/go-jsonschema => github.com/tjungblu/go-jsonschema v0.9.1-0.20210922142453-a1b781d84980
