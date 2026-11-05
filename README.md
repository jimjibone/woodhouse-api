# woodhouse-api

This repository contains the Woodhouse gRPC API protobuf and related files.

The API is stored in a major version directory. Only the current version is
kept: version 2 replaced version 1 outright, with no backwards compatibility.

## Version 2

This is the current and only version of the API. The protos are under
`proto/v2` (`clients`, `local`, `provisioning`) and the generated Go code under
`go/v2`. The schema version (see below) is separate and changes more often.

The API covers connections between the `core` server and either bridge/reactor
clients or user clients.

Bridges and Reactors are both known as clients of the server, with Reactors just
being an extension on top of the Bridge client API. This API is protected by a
pairing based authentication system, where users must approve the pairing
process. The API for clients is covered by the `AuthService` and `ClientService`.

User apps communicate via the `UserService` and `UserAuthService`. This provides
a typical user authentication service and methods for accessing the core server
in a user optimised way.

## Device model and service schema

A device has endpoints (one per physical or logical unit, `"0"` is the root),
an endpoint has services, and a service has attributes, commands and events.
See `Device`, `Endpoint` and `Service` in `proto/v2/clients/client_service.proto`.

`schema/services.yaml` defines every service type: its attributes, commands,
events and feature flags. It is the single source of truth for code
generators. The Go package `github.com/jimjibone/woodhouse-api/schema` embeds,
validates and exposes it; its tests check that the proto `Service.ServiceType`
and `Unit` enums match the schema. `schema/schemaspec` turns schema fields,
commands and events into API Specs; code generators use it at generate time,
so runtime code never parses the schema. `values` builds and reads values and
validates them against specs, with no dependency on the schema.

### Schema version

Every call to `ClientService` and `UserService` carries the gRPC metadata key
`woodhouse-schema-version` (`schema.HeaderName`) set to the `version` in
`schema/services.yaml` the client was built against (`schema.Version()`). The
server rejects a missing or different version, so a component built against an
older schema fails loudly instead of misreading devices. Bump the version on
any change to the schema or to the device model in the protos.

## Discovery

The core server advertises its user API over mDNS/Bonjour as
`_woodhouse._tcp`. The instance name is the server's instance name
(`Settings.instance_name`) and the port is the gRPC API port. The TXT record
carries:

| Key | Value |
|---|---|
| `fp` | Lowercase hex SHA-256 of the API's TLS leaf certificate (DER). Lets an app recognise a server it has already pinned, whatever its address. |
| `setup` | `1` on a woodhouse-os device that has not been adopted yet (no admin). Absent otherwise. Such a device is set up with the provisioning protocol, not by logging in. |

TXT records are unauthenticated: clients use them to match and label
servers, never in place of verifying the pinned certificate.

## Building

There is currently support for generating gRPC and protobuf code for the Go
language. This is simplified into a single command via the `Taskfile.yml` which
can be imported into parent projects (assuming one is using this project as a
submodule).

The resulting Go files are located in this directory under the `/go` subdirectory.

## Usage Examples

This repository is normally included as a submodule and build as part of a
larger project. In fact, you may not need to use this project directly as it is
already bundled in:

- [woodhouse-core](https://github.com/jimjibone/woodhouse-core) which uses it to
  implement the woodhouse core server and admin web interface.
- [wh](https://github.com/jimjibone/wh) which uses it to provide a Go client
  library for bridges and reactors.

## History

If you go looking through the git commit history you will notice that some
commits look slightly odd. This is because the files were extracted while
retaining commit history from the original monorepo which is now known as
woodhouse-core. This is to allow for future reasoning about why things are the
way they are. Git commit messages can be very useful if written well.
