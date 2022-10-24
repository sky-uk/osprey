# Release 2.11.0

- Run as nonroot (`nobody`/`65534` by default)
- Upgrade Alpine and all Go dependencies to latest versions

# Release 2.10.0

- Upgrade the version of a dependency due to issues reported upstream

# Release 2.9.0

- Enforce TLS verification when connecting to targets by default.  This can be overriden using the
  `skip-tls-verify` flag.

# Release 2.8.0

- Update imports for v2 compatibility

# Release 2.7.0

- Move to v2.0 of the `well-known/openid-configuration`. In turn this makes Osprey use v2.0 of the authorize
  endpoint. This adds support for more recent features of OIDC.

# Release 2.6.0

- Allow osprey client to retrieve the API server URL and CA cert from the GKE-specific
  OIDC ClientConfig resource. See the `use-gke-clientconfig` osprey config element.

# Release 2.5.0

- Add ability for osprey client to fetch the API server CA from the API server itself,
  rather than needing an osprey server deployment to serve it. See the Kubernetes feature
  gate RootCAConfigMap introduced in v1.13, which became enabled by default in 1.20.

# Release 2.4.0

- Add username/password command line options for Osprey login

# Release 2.3.0

- Automatically close browser window if possible for Azure login

# Release 2.2.0

- Add automatic browser popup for Azure login
- Fix error message for cluster-info req.
- Remove `latest` artifact from bintray.

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

- Update Alpine to 3.9 - fix vulnerability <https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2019-5021>.
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
