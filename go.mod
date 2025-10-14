module github.com/openshift/must-gather-clean

go 1.21

require (
	github.com/atombender/go-jsonschema v0.0.0-00010101000000-000000000000
	github.com/openshift/build-machinery-go v0.0.0-20230824093055-6a18da01283c
	github.com/schollz/progressbar/v3 v3.18.0
	github.com/spf13/cobra v1.8.0
	github.com/stretchr/testify v1.9.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/klog/v2 v2.110.1
	k8s.io/utils v0.0.0-20250820121507-0af2bda4dd1d
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/pkg/errors v0.8.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sanity-io/litter v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/term v0.28.0 // indirect
	gopkg.in/yaml.v2 v2.2.2 // indirect
)

replace github.com/atombender/go-jsonschema => github.com/tjungblu/go-jsonschema v0.9.1-0.20210922142453-a1b781d84980
