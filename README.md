# must-gather-clean

This tool is used for obfuscating sensitive data from [must-gather](https://github.com/openshift/must-gather) dumps.

# Running the tool

`must-gather-clean` should be pointed to the root folder of an already generated must-gather.

So let's say you ran:

> oc adm must-gather --dest-dir=must-gather-output

Then cleaning can be done by running:

> must-gather-clean -c config.yaml -i must-gather-output -o must-gather-output-cleaned

The cleaned must-gather can then be found in the `must-gather-output-cleaned` folder, indicated by the `-o` argument.

# Configuration

A very basic default configuration you can supply as the above `-c` flag for OpenShift can be found
under [examples/openshift_default.yaml](examples/openshift_default.yaml). 

In any case, if you want to obfuscate your domain names (e.g. DNS entries), then you have to adjust the list of `domainNames` to include yours.

In case you don't need networking or SDN information in the must-gather, you can run the configuration under [examples/openshift_omit_network.yaml](examples/openshift_omit_network.yaml). 
This will ignore the largest files that also take a long time to obfuscate.

The fully supported schema defined in [JSON schema](https://json-schema.org/)
can be found in [schema.json](pkg/schema/schema.json) along with examples and documentation. A more browsable
alternative can be found here
on [json-schema.app](https://json-schema.app/view/%23?url=https%3A%2F%2Fraw.githubusercontent.com%2Fopenshift%2Fmust-gather-clean%2Fmain%2Fpkg%2Fschema%2Fschema.json).