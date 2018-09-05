# Release 1.2.0
- Handles `groups` for targets, allowing client commands to target a
  specific group.
- Adds client commands: `config`, `config targets`.
- All actions and outputs are now done against targets in alphabetical
  order.
- Adds osprey `--version` flag to provide build information.

# Release 1.1.0
- Adds support for `certificate-authority-data` in client config.

# Release 1.0.0
- Includes client commands: `user`, `user login`, `user logout`.
- Includes server command: `serve`.
- Handles `aliases` for targets, generating an extra context with the
  alias name.
- Uses `ospreyconfig` target names as the names for `kubeconfig`'s cluster,
  context and user names.
- Adds a global `certificate-authority` overridable by target specific
  ones.
- Releases binaries for Windows, Linux and Mac to Bintray.
- Releases an alpine based docker image to Docker Hub.
- Requires Dex `skipApprovalScreen` to be true.
- Only supports a single-connector Dex configuration.
  Tested with:
  - [LDAP](https://github.com/coreos/dex/blob/master/Documentation/connectors/ldap.md)
