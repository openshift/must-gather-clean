# must-gather-clean

This tool is used for obfuscating sensitive data from [must-gather](https://github.com/openshift/must-gather) dumps.

# Running the tool

`must-gather-clean` should be pointed to the root folder of an already generated must-gather.

So let's say you ran:

> oc adm must-gather --dest-dir=must-gather-output

Then cleaning can be done by running:

> must-gather-clean -c config.yaml -i must-gather-output -o must-gather-output-cleaned

The cleaned must-gather can then be found in the `must-gather-output-cleaned` folder, indicated by the `-o` argument.

