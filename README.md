# woodhouse-api

This repository contains the Woodhouse gRPC API protobuf and related files.

The API is stored in major version directories, with there currently only being
version 1. In the future we may introduce new versions, so this separation can
2. enable backwards compatibility if we choose to support it.

## Version 1

This is the current and only version of the API and all files can be found under
`v1/clients`.

The API covers connections between the `core` server and either bridge/reactor
clients or user clients.

Bridges and Reactors are both known as clients of the server, with Reactors just
being an extension on top of the Bridge client API. This API is protected by a
pairing based authentication system, where users must approve the pairing
process. The API for clients is covered by the `AuthService` and `ClientService`.

User apps communicate via the `UserService` and `UserAuthService`. This provides
a typical user authentication service and methods for accessing the core server
in a user optimised way.

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
