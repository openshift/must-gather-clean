module github.com/openshift/must-gather-clean

go 1.16

require (
	github.com/atombender/go-jsonschema v0.0.0-00010101000000-000000000000
	github.com/openshift/build-machinery-go v0.0.0-20210712174854-1bb7fd1518d3
	github.com/spf13/cobra v1.1.3
	github.com/stretchr/testify v1.7.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/klog/v2 v2.10.0
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/atombender/go-jsonschema => github.com/tjungblu/go-jsonschema v0.9.1-0.20210811141250-01318803193d
