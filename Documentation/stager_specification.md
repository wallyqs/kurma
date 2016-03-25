# Kurma Stager Specification

Stagers in Kurma are used to provide a pluggable interface to control pod
orchestration and supervision.

When a pod is launched, the Kurma daemon will resolve all of the dependencies
for the applications in the pod and provide them in the form of a stager
manifest and bind mount them into the stager's filesystem. This way, a stager
has everything it needs for launching the workloads.

## Prerequisites

A stager itself is just an ACI image and is loaded in just like any other
image. In order for an image to be used as a stager, it must meet the following:

1. The image cannot have any dependencies. A stager should not need any overlay
   filesystems set up, since the Kurma daemon does not handle any union
   filesystems. This is the responsibility of the stager for its apps.
2. The image must have a signature with a key that is known and trusted by the
   Kurma daemon. This list of keys is defined in its configuration file and only
   managable there. Since a stager can execute with host privilege, it must
   ensure the image can be executed in that context first.

## Execution

When a stager is executed, it will be launched chrooted within a directory
containing its root filesystem. This ensures its filesystem has all of its
dependencies and avoids and mismatches with the host's filesystem.

When the stager is launched, it will be in its own mount namespace. It will
typically have its own network namespace, unless the storm is configured to
share the host's network namespace. The executable will be the `exec`
setting of the stager's AppC Image Manifest.

The mount namespace is configured to be private. This ensures that any mounts
made within the stager or its applications will not propagate to the host.

The network namespace will be preconfigured with the networking devices for the
container. Note that it may be the host's network namespace, if the pod is
supposed to be using the host's networking.

## Filesystem Configuration

The filesystem for the stager will be pre-populated with everything the stager
will need for the pod and its applications. The following paths will be created
and stagers should ensure they don't include anything related to the following
paths.

* `/manifest` - This contains the JSON manifest containing the information
  needed for the stager. See the "Stager Manifest" section for the format.
* `/layers/*` - This directory contains read-only bind mounts to the extracted
  root filesystem of each of the layers needed for the applications in the
  pod. They are named with the
  [AppC Image ID](https://github.com/appc/spec/blob/master/spec/aci.md#image-id)
  in the form of `sha512-[checksum]`. The bind mounts are all read-only, as the
  stager and the applications should not be modifying these. The stager should
  be using these as a base for setting up the app filesystem with either a union
  filesystem or copying the layers.
* `/volumes/*` - This directory contains all of the volumes referenced by the
  PodManifest. The name of the directory will match the name of the volume from
  the PodManifest

The stager will be chrooted within its root directory and contain each of the
items referenced above. The following additional elements will be mounted:

* `/dev` from the host will be bind mounted in.
* `/proc` will be mounted for the stager.
* `/sys` will be mounted read only.
* `/sys/fs/cgroups` will be mounted read-write, scoped to just the stager's
  cgroup.

## Lifetime Management

The stager is expected to stay alive for the lifetime of the pod. If at any
point the stager exits, Kurma will consider the pod errored and tear it and any
remaining processes down.

When the stager is launched, it will be passed an additional file handler as
descriptor `4`. The descriptor is expected to be closed once the stager has
finished setting up the workloads and the pod is considered running.

## Stager Manifest

The stager manifest is a JSON document that contains the information necessary
to configure and run the applications within the pod, and to match up the
provided filesystem information to its place within the pod.

At the top level, the structure is as follows:

```
{
    "kurmaVersion": "0.4.1",
    "name": "example1",
	"pod": { },
	"images": { },
	"appImageOrder": { },
	"stagerConfig": { }
}
```

* `kurmaVersion` - The `kurmaVersion` will contain the version of Kurma that was
  used at the time the pod was launched. This is primarily used when `kurmad` is
  in use and to ensure compatibility when the daemon is upgraded independent of
  the pod.
* `name` - The `name` element is the string name that was given to the pod. It
  could optionally be used by the stager to configure the hostname in the
  applications.
* `pod` - The `pod` element contains the
  [AppC Pod Manifest](https://github.com/appc/spec/blob/master/spec/pods.md)
  object and the definition for the applications in the pod as provided.
* `images` - The `images` element contains a map of the Image ID for an image to
  the
  [AppC Image Manifest](https://github.com/appc/spec/blob/master/spec/aci.md).
* `appImageOrder` - The `appImageOrder` element contains a map of the
  application name from the Pod Manifest and a string array containing the Image
  IDs of all the images that make up its filesystem, with the first element
  being the top most image, and the last being the lower most.
* `stagerConfig` - The `stagerConfig` element is a JSON document containing what
  ever configuration was provided to Kurma. This is specific to the stager and
  provides a way to pass down configuration parameters from the administrator to
  inform the stager. For instance, in the default configuration, it will pass
  over whether to use overlay or aufs for the union filesystem.

An example document is:

```
{
    "kurmaVersion": "0.4.1",
	"pod": {
		"acVersion": "0.7.4",
		"acKind": "PodManifest",
		"apps": [{
			"name": "nats",
			"image": {
				"id": "sha512-de8f22333d0234270a8a18d47dcc475a69489ab18bf7e7fbbcdee50b6d0a1c8f536750c5c55e9a0e63574053f2fc51bbba20caedb14220e0c157e6d8fb35f4fc"
			}
		}]
	},
	"images": {
		"sha512-de8f22333d0234270a8a18d47dcc475a69489ab18bf7e7fbbcdee50b6d0a1c8f536750c5c55e9a0e63574053f2fc51bbba20caedb14220e0c157e6d8fb35f4fc": {
			"acKind": "ImageManifest",
			"acVersion": "0.7.0",
			"name": "registry-1.docker.io/library/nats",
			"labels": [{
				"name": "version",
				"value": "latest"
			}, {
				"name": "os",
				"value": "linux"
			}, {
				"name": "arch",
				"value": "amd64"
			}],
			"app": {
				"exec": ["/gnatsd", "-c", "/gnatsd.conf"],
				"user": "0",
				"group": "0",
				"ports": [{
					"name": "4222-tcp",
					"protocol": "tcp",
					"port": 4222,
					"count": 1,
					"socketActivated": false
				}, {
					"name": "6222-tcp",
					"protocol": "tcp",
					"port": 6222,
					"count": 1,
					"socketActivated": false
				}, {
					"name": "8222-tcp",
					"protocol": "tcp",
					"port": 8222,
					"count": 1,
					"socketActivated": false
				}]
			},
			"annotations": [{
				"name": "authors",
				"value": "Derek Collison \u003cderek@apcera.com\u003e"
			}, {
				"name": "created",
				"value": "2015-12-09T20:18:21Z"
			}, {
				"name": "appc.io/docker/registryurl",
				"value": "registry-1.docker.io"
			}, {
				"name": "appc.io/docker/repository",
				"value": "library/nats"
			}, {
				"name": "appc.io/docker/imageid",
				"value": "f5c45d5f9cacc583e02dbc3cba8b5e35cb054334fc8db24f3a51e8873839af48"
			}, {
				"name": "appc.io/docker/parentimageid",
				"value": "e5da1391e6bdf3ec19c5f2216b2af8056a23d3cdb802f56a738cdb7ee230cda6"
			}]
		}
	},
	"appImageOrder": {
		"nats": ["sha512-de8f22333d0234270a8a18d47dcc475a69489ab18bf7e7fbbcdee50b6d0a1c8f536750c5c55e9a0e63574053f2fc51bbba20caedb14220e0c157e6d8fb35f4fc"]
	},
	"stagerConfig": {}
}
```

## Call Ins

There are a number of functions that Kurma may be calling in to the stager to
perform on an ongoing basis. These are instrumented through additional
executables. These are done by Kurma calling them by entering the mount and
network namespace. If the executables need to enter other namespaces, they must
instrument that.

The following executables are expected within the stager:

* `/opt/stager/status` to check the status of the applications in the pod.
* `/opt/stager/logs` to request log files for an application within the pod.
* `/opt/stager/run` to run a new process within an application in the pod.
* `/opt/stager/attach` to attach to the input/output of an application.

#### `status`

The `status` command is called to get the current pod state. It is not called on
a regular pole interval, but it is triggered by an API call to get the pod's
internal status.

It takes no information in, it is expected just to output JSON over stdout with
the pod's application status, primarily accounting for whether the applications
are running and if not, what their exit code was.

The response is in the following format when the pod is in a steady state:

```
{
	"nats": {
        "pid": 1234,
        "exited": false
	}
}
```

For an exited application, it would return `running` of `false` and an `exitCode`.

```
{
	"nats": {
        "exited": true,
		"exitCode": 1,
		"exitReason": "killed"
	}
}
```

#### `logs`

#### `run`

The `run` command is used to execute a specified command within one of the
applications in the pod. A single command line argument will be passed with the
name of the application to run the command in.

The settings for the command to run will be passed over a separate file
descriptor in JSON form and will always be available on file descriptor 3.

The JSON input is similar to the AppC Spec's "App" field of the
ImageManifest. It could look like the following:

```json
{
    "exec": [ "/bin/bash" ],
    "user": "0",
    "group": "0",
    "supplementaryGIDs": [ 20, 21 ],
    "workingDirectory": "/",
    "environment": [
        { "name": "PATH", "value": "/usr/bin:/usr/sbin:/bin:/sbin" }
    ],
    "tty": true
}
```

* **exec** (list of strings, required) executable to launch and any flags
* **user**, **group** (string, required) indicates either the username/group
  name or the UID/GID the app is to be run as (freeform string). The user and
  group values may be all numbers to indicate a UID/GID, however it is possible
  on some systems (POSIX) to have usernames that are all numerical. The user and
  group values will first be resolved using the image's own `/etc/passwd` or
  `/etc/group`. If no valid matches are found, then if the string is all
  numerical, it shall be converted to an integer and used as the UID/GID. If the
  user or group field begins with a "/", the owner and group of the file found
  at that absolute path inside the rootfs is used as the UID/GID of the
  process. Example values for the fields include `root`, `1000`, or
  `/usr/bin/ping`.
* **supplementaryGIDs** (list of unsigned integers, optional) indicates
  additional (supplementary) group IDs (GIDs) as which the app's processes
  should run.
* **workingDirectory** (string, optional) working directory of the launched
  application
* **environment** (list of objects, optional) represents the app's environment
  variables. The listed objects must have two key-value pairs:
  **name** and **value**. The **name** must consist solely of letters, digits,
  and underscores '_' as outlined in
  [IEEE Std 1003.1-2001](http://pubs.opengroup.org/onlinepubs/009695399/basedefs/xbd_chap08.html). The
  **value** is an arbitrary string. These values are not evaluated in any way,
  and no substitutions are made.
* **tty** (boolean, optional, defaults to "false" if unsupplied) if set to true,
  a tty will be allocated and given to the process as its stdin/stdout/stderr.

It is expected that the JSON configuration should be read in within 10
seconds. Failure to read all of the configuration, including the EOF, will
result in the `run` command being torn down and an error being returned.

The command's stdin/stdout/stderr should be passed along directly to the child
command within the pod's application.

The `run` command is expected to either exec or block until the child command is
complete. The exit code should be propagated back to the parent.

Example:

```shell
$ /opt/stager/run ubuntu
```

#### `attach`

The `attach` command is used to attach the executor to stdin/stdout/stderr of an
application within the pod. The command will be called with an argument for
which application should be attached to.

Example:

```
$ /opt/stager/attach nats
```

The command should take stdin/stdout/stderr of the `attach` command and connect
it to stdin/stdout/stderr of the command executed for the specified application.

## Considerations

There are a number of important considerations that a stager should be aware of
and chose how it manages.

* Mount and network namespaces are created by the top level daemon. If the
  stager is using user namespaces, the owner of non-user namespaces is important
  to ensure isolation. It may be necessary for the stager to create another
  mount and network namespace under the user namespace and transfer the
  networking to it. See "Interaction of user namespaces and other types of
  namespaces" in
  [user_namespaces(7)](http://man7.org/linux/man-pages/man7/user_namespaces.7.html).
* The stager is responsible for lifecycle management within the pod.
* The stager controls the security scoping for the applications it runs. If the
  stager takes over PID 1 within a container, and it is implementing shared PID
  namespaces between apps, it should be aware of things like traversal through
  `/proc`.
  * Using `/proc/1/root/`, a container could access the root filesystem of the
    stager. Generally, this only contains the pod's data, so it may just be
    leaking data across applications. It should manage that.
  * With shared namespaces, be aware of where it may be possible to dump memory
    and see information.
* Additional call in functions may need to be entering additional namespaces,
  and should be contious of when they enter into the purview of the applications
  in the pod.
