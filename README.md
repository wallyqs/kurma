[![License][License-Image]][License-URL] [![Build][Build-Status-Image]][Build-Status-URL] [![Release][Release-Image]][Release-URL]

# Kurma

Kurma is a container runtime built with extensibility and flexibility in
mind. It focuses on "everything is a container" and uses this to enable plugins
that run within Kurma, leaving Kurma easy and simple to deploy and
manage. Configuring networking plugins or customizing how containers are
instrumented is easily extensible.

Kurma implements the [App Container (appc)](https://github.com/appc/spec)
specification, and leverages
[libcontainer](https://github.com/opencontainers/runc/tree/master/libcontainer)
from the [Open Container Initiative (OCI)](https://www.opencontainers.org/).

For more information about Kurma, please visit our [website](https://kurma.io)
and see our [documentation](https://kurma.io/documentation).

### Building Kurma

See the
[KurmaOS Repo](https://github.com/apcera/kurmaos/blob/master/README.md#build-process)
for instructions on how to build Kurma.

### Downloading Kurma

The latest release images can be [on our website](https://kurma.io/download).

[License-URL]: https://opensource.org/licenses/Apache-2.0
[License-Image]: https://img.shields.io/:license-apache-blue.svg
[Build-Status-URL]: http://ci.kurma.io
[Build-Status-Image]: https://img.shields.io/teamcity/http/ci.kurma.io/s/Kurma_UnitTests_2.svg
[Release-URL]: https://kurma.io/download
[Release-Image]: https://img.shields.io/badge/release-v0.3.3-1eb0fc.svg
