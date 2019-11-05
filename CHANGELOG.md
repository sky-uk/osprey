# Release 2.1.0
- Adds a public constructor to initialise a Target struct.
- Fix release bintray deployment

# Release 2.0.0
- Adds support for authentication using Azure Active Directory as the OIDC.
- This release introduces a breaking change to osprey config files. This is due to functionality to support multiple
  identity providers in the same configuration file.

# Release 1.5.0
- Make readiness depend on the state of the oidc provider state

# Release 1.4.0
- Update Alpine to 3.9 - fix vulnerability https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2019-5021.
- Update golint dependency to golang.org/x/lint/golint.
- Update error message during login.
- Fix broken functionally to allow osprey server to start without TLS.
- Fix `latest` artifacts publishing.
- Fix `prepare-release-bintray` make target.

# Release 1.3.0
- Fixes displaying a target multiple times if the targets existed in
  multiple groups.

# Release 1.2.0
- Handles `groups` for targets, allowing client commands to target a
  specific group.
- Adds client commands: `config`, `config targets`.
- All actions and outputs are now done against targets in alphabetical
  order.
- Adds osprey `--version` flag to provide build information.
- Keep custom context information after login. ([#17](https://github.com/sky-uk/osprey/issues/17))

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
  - [LDAP](https://github.com/dexidp/dex/blob/master/Documentation/connectors/ldap.md)
