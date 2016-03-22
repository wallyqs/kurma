# Kurma Network Specification

Networking within Kurma is implemented to be fully pluggable and implements a
simple interface to allow networking modes to be added without any changes or
extensions to the base daemon.

One of the core tenants of Kurma is that everything is a container. This means
that Kurma is self encompassing, has limited host dependencies, and easily
extensible through deploying additional containers for extensibility.

Networking plugins are packaged as containers that get deployed to a Kurma host.

## Instrumentation

Within Kurma, it will instrument a networking pod which contains a set of
containers which each correspond to a network definition. The networking pod is
setup such that the containers within it have elevated host priviledge in order
for them to be able to configure networking on other pods.

When Kurma is launching a new pod, it can be given a list of networks it should
be associated with. Kurma will then call in to the containers corresponding to
those networks within the networking pod and have them set up the networking.

## Kurma Configuration

Kurma's configuration includes a `pod_networks` section with specifies a list of
available networks, where to locate the container image for them, and their own
configuration.

```
{
  "pod_networks": [
    {
      "name": "mynet",
      "aci": "apcera.com/kurma/cni-netplugin",
      "default": true,
      "containerInterface": "eth0",
      "type": "bridge",
      "bridge": "mynet0",
      "isGateway": true,
      "ipMasq": true,
      "ipam": {
        "type": "host-local",
        "subnet": "10.10.0.0/16",
        "routes": [ { "dst": "0.0.0.0/0" } ]
      }
    }
  ]
}
```

The above example configures both loopback and a `mynet` network for
containers. The parts of the configuration that are relevant to Kurma include:

* `name` - This specifies a unique identifier for the network. When pods are
  launched, they can include a list of networks to attach and uses this name.
* `aci` - This specifies how to locate the image for the networking
  plugin. Options for this field include:
  * `file:///path/to/image.aci` - A `file://` uri can be used to specify an
    image on the host's filesystem.
  * `http://example.com/image.aci` - A `http://` or `https://` uri can be used
    to specify a URL to retrieve the image.
  * `docker:///user/plugin-name:tag` - The `docker://` uri can be used to
    retrieve a Docker image from a repository. If using a repository other than
    the Docker Hub, specify the hostname in the URL.
* `default` - This specifies whether Kurma should attach a new pod to this
  network by default. These will be used only when there isn't a specific set of
  networks being requested for the pod.
* `containerInterface` - This specifies the name of the interface to configure
  within the container. It can be omitted on plugins where it isn't relevant,
  such as with the loopback plugin. However Kurma can dynamically generate it
  with some templatizing options and will ensure no collisions are created of
  the container's known interfaces.

The rest of the JSON for configuration is passed along to the network plugin and
can include any options specific to it.

The format is generally aligned with the
[Container Network Interface (CNI)](https://github.com/appc/cni) schema from the
AppContainer set of specificiations. Many of the default plugins available with
Kurma are networking plugins wrapping the CNI binaries. For more on the
configuration of the CNI plugins themselves, see the
[CNI documentation](https://github.com/appc/cni/tree/master/Documentation).

## The Container and The API

Kurma sets up a specific networking pod which contains containers for all of the
networks defined. It configures the pod to ensure that they share the host's
IPC, network, and UTS namespace. This helps to ensure that the network plugins
are able to configure the network on both the host's side. It will also attach a
specific volume which is used to contain persistent references to network
namespaces that are created for newly launched pods. This can be used for a
plugin to `setns` into the network namespace for the target pod and configure
the interface on its side.

The network plugins will still have its own mount namespace and have its own
filesystem available to it.

The plugins work by instrumenting three executables within the container
image. They are:

* `/opt/network/add` to configure the networking on a new container.
* `/opt/network/del` to deprovision/cleanup when a container shuts down.

These scripts will be invoked as the root user to ensure they have access to
configure both the host and the container.

With all executables, the configuration for the plugin will be passed in over
`stdin`. Command line arguments are also provided with information on the
pod being set up.

#### `add`

The `add` step is called when a new container is being provisioned. It will
provide the plugin's configuration over `stdin` and is passed three arguments on
the command line. These are the full path to the new container's network
namespace, the container's UUID, and the interface within the container to
configure, if one was provided within the configuration. Since the
`containerInterface` section of the configuration may be a template, the
rendered version will be provided as the argument.

Example:

```
$ /opt/network/add /proc/123/ns/net 146d7cef-fbf6-41da-a2f8-eba218597f9c eth0
```

On success, Kurma will expect it to exit `0` and return a CNI `Result` object
containing the IP information provisioned on the container over `stdout`.

```
{
  "ip4": {
    "ip": <ipv4-and-subnet-in-CIDR>,
    "gateway": <ipv4-of-the-gateway>,  (optional)
    "routes": <list-of-ipv4-routes>    (optional)
  },
  "ip6": {
    "ip": <ipv6-and-subnet-in-CIDR>,
    "gateway": <ipv6-of-the-gateway>,  (optional)
    "routes": <list-of-ipv6-routes>    (optional)
  }
}
```

The response will be added onto the metadata for the container.

Any non-zero exit code will be viewed as an error, and `stdout`/`stderr` will be
read for the error message.

Any state that needs to be tracked for the container should use the container's
UUID as the key for the container, as it is provided on `del` as well and
ensured to be unique.

The `add` step may be called concurrently for separate containers being set
up. The script should be aware of this and account for any file or state locking
that may be necessary.

The executable is given an upper limit of 1 minute to return, otherwise it will
be considered errored. This won't result in the network plugin being torn down.

#### `del`

The `del` step is called when a container is being shut down. It can be used to
cleanup any state, such as IP address reservations, drop iptables rules, or
teardown the interface within the container.

It will provide the plugin's configuration over `stdin` and is passed three
arguments on the command line. These are the full path to the new container's
network namespace, the container's UUID, and the interface within the container
to configure, if one was provided within the configuration. Since the
`containerInterface` section of the configuration may be a template, the
rendered version will be provided as the argument.

Example:

```
$ /opt/network/del /proc/123/ns/net 146d7cef-fbf6-41da-a2f8-eba218597f9c eth0
```

No response is expected from the `del` step. It will expect that exiting `0` is
a success, and any exit with a non-zero exit code will be an error. On error
`stdout`/`stderr` will be read for the error message. Erroring will not result
in tearing down the network plugin.

The `del` step may be called concurrently for separate containers being torn
down. The script should be aware of this and account for any file or state
locking that may be necessary.

The executable is given an upper limit of 1 minute to return, otherwise it will
be considered errored.

## Main Executable

The ACI image for the networking plugin is expected to have a main executeable
defined in the `exec` field of the `app` section. This program will be ran on
start of the plugin and is expected to remain running indefinitely. If it exits,
Kurma will assume the plugin has failed.

The main executable is intended to act as a setup point for the container. It
will be given the plugin's configuration over `stdin` and provides no arguments
as with the other executables. It does not expect any output.

If the networking plugin needs any background processes running, it can execute
them if necessary.

## Special Considerations

Networking plugins should take in a few pieces for consideration to ensure they
can work along side other plugins.

First, ensure its configuration takes in necessary pieces to avoid name
collisions on the host. For instance, the CNI bridge plugin takes in the name of
the bridge to create. This way, the administrator defining the plugins can
configure them to avoid collision.

Second, multiple plugins may be configuring shared areas like iptables. When
doing this, plugins should always aim to use their own named rules. This ensures
quick cleanup, where they can just drop a table for a container. Also be aware
of rule inputs to ensure rules aren't too generalized and may match traffic not
intended for them.
