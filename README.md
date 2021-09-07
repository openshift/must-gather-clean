# must-gather-clean

This tool is used for obfuscating and omitting sensitive data from [must-gather](https://github.com/openshift/must-gather) dumps.

# Running the tool

`must-gather-clean` should be pointed to the root folder of an already generated must-gather.

So let's say you ran:

> oc adm must-gather --dest-dir=must-gather-output

Then cleaning can be done by running:

> must-gather-clean -c config.yaml -i must-gather-output -o must-gather-output-cleaned

The cleaned must-gather can then be found in the `must-gather-output-cleaned` folder, indicated by the `-o` argument.

# Configuration

A very basic default configuration you can supply as the above `-c` flag for OpenShift can be found
under [examples/openshift_default.yaml](examples/openshift_default.yaml). If you want to obfuscate your domain names (e.g. DNS entries), then you have to adjust the list of `domainNames` to include yours.

In case you don't need networking or SDN information in the must-gather, you can run the configuration under [examples/openshift_omit_network.yaml](examples/openshift_omit_network.yaml). 
This will ignore the largest files that also take a long time to obfuscate.

The configuration is explained along examples below, a fully supported schema defined in [JSON schema](https://json-schema.org/)
can be found in [schema.json](pkg/schema/schema.json) along with more examples and documentation. A more browsable
alternative can be found here
on [json-schema.app](https://json-schema.app/view/%23?url=https%3A%2F%2Fraw.githubusercontent.com%2Fopenshift%2Fmust-gather-clean%2Fmain%2Fpkg%2Fschema%2Fschema.json).

## Obfuscation

The configured obfuscation logic will be applied to each non-omitted file and on a line-by-line basis.
Let's start with a very minimal obfuscation example:

```
config:
  obfuscate:
  - type: MAC
```

This configuration applied on a `must-gather` will detect all MAC addresses recursively in all the files. 
Since obfuscation is about replacing the found information, the above will simply replace the found MAC address with `xx:xx:xx:xx:xx:xx`. We call this a `Static` replacement, which is the default for all types of obfuscators.

There is another type of replacement called `Consistent`, that can be configured like this:

```
config:
  obfuscate:
  - type: MAC
    replacementType: Consistent
```

This will detect all MAC addresses and replace them with a "consistent" identifier that looks like this `xxx-mac-000001-xxx`. 
Let's say one of your eth interfaces has the mac address `52:54:00:5e:ee:c6` and was logged, then `must-gather-clean`  will guarantee that it will always be assigned the same obfuscated consistent identifier across all files in a must-gather. 
That primarily helps our support and engineers to ensure we can still understand and reproduces challenges that you were facing without putting your classified information at risk.

Let's look at another obfuscation type named `IP`. This type can be used to clean IP addresses, we support both IPv4 and IPv6 except for local interfaces (`127.0.0.1`, `0.0.0.0` and `::1`) that will be preserved.

You can configure this along with the MAC obfuscator like this:

```
config:
  obfuscate:
  - type: MAC
    replacementType: Consistent
  - type: IP
    replacementType: Consistent
```

On a line-by-line basis, this will always execute the MAC obfuscation first and then the IP obfuscator - we'll go through this behaviour in more detail in the `Chaining obfuscators and side effects` section below. 

Another configuration flag that we support for each obfuscation type is the `target`. The target is useful when the confidential information can be found not only in the file content, but also in the folder or file names. 
This can very frequently happen with IP addresses, for example through node names. You can control that independently for each type like this:

```
config:
  obfuscate:
  - type: MAC
    replacementType: Consistent
    target: FileContents
  - type: IP
    replacementType: Consistent
    target: FilePath
```

As you can see, the MAC obfuscator would work on file content whereas the IP obfuscator would work only on FilePaths. There is a mixed target called `All`, that will obfuscate on both paths and contents. 
The default if no target is specified is `FileContents`, it is thus always recommended using the IP obfuscator with `target: All` to not accidentally leak IP information through folder names.

The third built-in type of obfuscation is `Domain`, let's take a look how this can be configured:

```
config:
  obfuscate:
  - type: Domain
    domainNames:
    - "rhcloud.com"
    - "dev.rhcloud.com"
```

As you can see, this type must be customized by supplying domain names. 
Kubernetes resources are defined along with their domain names (eg "apps.openshift.io/v1") and thus would be automatically recognized as such and obfuscated as a false-positive.
We thus kindly ask the user to supply their confidential domain names manually through the configuration.

The above definition will obfuscate `rhcloud.com` as `domain0000001` (consistent) or as `xxxxxxxxxxxxx` (static). 
Note that this does not include subdomains, they would need to be separately obfuscated. 
A domain name defined as `staging.rhcloud.com` would only be obfuscated as `staging.domain0000001`, thus, it makes sense to also include all common subdomains as done in the example with "dev".

### Custom Obfuscations

Aside from the above three built-in types to obfuscate, we also offer custom obfuscators that allow users to fine-tune the replacement of certain strings (eg custom auth token formats, confidential domain knowledge or keywords).
This comes through two custom types, one is replacement via type `Keywords` the other via type `Regex`.

#### Keywords

Let's start by looking into `Keywords` first:

```
config:
  obfuscate:
  - type: Keywords
    replacement:
       hello: bye
       tomorrow: yesterday
```

This configuration will simply replace the strings on the left-hand side with the values on the right-hand side. 
The `target` variable here is supported as well, so you can also target specific files and obfuscate their name, for example:

```
config:
  obfuscate:
  - type: Keywords
    target: FilePath
    replacement:
       namespaces: virtual-cluster
```

which would replace the ubiquitous "namespaces" folder to be called "virtual-cluster".

FilePath obfuscation works on the whole path (including the file name), so you can even obfuscate multi-level folder structures like this:

```
config:
  obfuscate:
  - type: Keywords
    target: FilePath
    replacement:
       namespaces/kube-system/apps: virtual-cluster/apps
```
which would condense the `namespaces/kube-system/apps` folder to become `virtual-cluster/apps` and all of its files would be under that new folder.

Since the replacement is already supplied, configuring the `replacementType` will have no effect.

#### Regex

Another common approach to detect strings by their format is using regular expressions. Internally this uses the [Golang regexp package](https://pkg.go.dev/regexp) if you need further details on how to express a pattern. 

Let's take a brief example on both FilePath and FileContents:

```
config:
  obfuscate:
  - type: Regex
    target: FilePath
    regex: "release-4\..*\/ingress_controllers\/.*\/haproxy.*"
  - type: Regex
    target: FileContents
    regex: ".*ssl-min-ver TLSv1.2$"
```

The first regex would match a path like: `release-4.1/ingress_controllers/something/haproxy.log` and would `x` out the whole path. 
The resulting filename would literally be: `xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`, thus there is very little practical use for using the regex like that.

This however, is much more useful in the second example where we want to obfuscate that we were using TLSv1.2 as the min version - which would also be replaced as `xxxxxxxxxxxxxxxxxxxxxxx`.

There is currently no support for consistent replacement as in the built-in types, there is a feature upcoming for capture groups and individual replacements thereof. 


#### Chaining obfuscators and side effects

As seen above, obfuscators can be chained and are guaranteed to be executed in order of definition on a line of text. This can cause some interesting side effects you should be aware of.
Let's take the following contrived example to illustrate:
```
config:
  obfuscate:
  - type: Keywords
    replacement:
       a: b
  - type: Keywords
    replacement:
       b: 192.168.2.1
  - type: IP
    replacementType: Static       
```

Running the above obfuscation on the string `a wonderful evening to go dancing` would yield the following result:
`xxx.xxx.xxx.xxx wonderful evening to go dxxx.xxx.xxx.xxxncing`, which might be counter-intuitive.
So what happened here? First off, we would replace all `a` with a `b`, that `b` in turn would be replaced with `192.168.2.1` that later matches as an IPv4 and gets obfuscated in a static manner. 

You can ensure that this does not happen, by providing custom obfuscators at the very bottom of the definition, preferably after all built-ins, and by ensuring you match on very specific terms (eg by supplying word boundaries in regular expressions).

## Omission

TODO(thomas)