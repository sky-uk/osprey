# Osprey

Client and service for providing OIDC access to Kubernetes clusters.

The client provides a `user login` command which will request a `username`
and a `password` and forward them to the service. The service will forward
the credentials to an OpenId Connect Provider (OIDC) to authenticate the
user and will return a JWT token with the user details. The token along
with some additional cluster information will be used to generate the
`kubectl` configuration to be used to access Kubernetes clusters.

### Supported OIDC providers
##### Dex
This implementation relies on one specific configuration detail of the OIDC
provider `SkipApprovalScreen : true` which eliminates the intermediate step
requiring a client to explicitly approve the requested grants before the
token is provided. If the target provider does not support this feature,
additional work is required to handle that approval.

##### Azure
When Azure is configured as the OIDC provider, the `user login`
command will generate a link to visit, which the user must open in a browser
in order to authenticate. Upon a successful login, the browser will send a
request to a local endpoint served by the osprey application. With the
information contained in this request it is able to request a JWT token
on your behalf.

## Quick links
- [Installation](#installation)
- [Client](#client)
- [Server](#server)
- [Examples](#examples)

## Installation

Osprey is currently supported on Linux, macOS and Windows. It can be
installed as a standalone executable or run as a container (only
Linux).
The Docker container is aimed to be used for the server side, while the
binaries' main use are the client commands.

### Binaries

There is currently no binary install option for Osprey, due to the
sunsetting of our old binary host.  You will need to build from source
or install with Docker.

For example:
```yaml
$ go install github.com/sky-uk/osprey/v2@v2.8.1

$ ~/go/bin/osprey --help
User authentication for Kubernetes clusters
...
```

### Docker

The Docker image is based on [Alpine Linux](https://hub.docker.com/_/alpine).
It can be pulled from our [Docker Hub repository](https://hub.docker.com/r/skycirrus/osprey/)

To pull a specific version replace `<version>` with the release version
(e.g `v9.9.0`; mind the `v` prefix).

```sh
$ docker pull skycirrus/osprey:<version>
```

# Client

The `osprey client` will request the user credentials and generate a
kubeconfig file based on the contents of its [configuration](#client-configuration).

To get the version of the binary use the `--version` argument:
```
$ osprey --version
osprey version 2.8.1 (2022-03-17T16:25:26Z))
```

You can run Osprey client in Docker by bind-mounting your Osprey config and kubeconfig files:

```
$ docker run --rm \
    --network=host \
    --env HOME=/ \
    -v $HOME/.config/osprey/config:/.config/osprey/config:ro \
    -v $HOME/.kube/config:/.kube/config \
    skycirrus/osprey:v2.8.1 user login
```

## Client usage
- [config](#config)
- [groups](#groups)
- [login](#login)
- [logout](#logout)
- [targets](#targets)
- [user](#user)

With a [configuration](#client-configuration) file like:
```yaml
providers:
  osprey:
    targets:
      local.cluster:
        server: https://osprey.local.cluster
      foo.cluster:
        server: https://osprey.foo.cluster
        alias: [foo]
        groups: [foo, foobar]
      bar.cluster:
        server: https://osprey.bar.cluster
        groups: [bar, foobar]
```

The `groups` are labels that allow the targets to be organised into categories.
They can be used, for example, to split non-production and production clusters
into different groups, thus making the interaction explicit.

Most of the client commands accept a `--group <value>` flag which indicate
Osprey to execute the commands only against targets containing the specified
value in their `groups` definition.

A `default-group` may be defined at the top of the configuration which will
apply that group to any command if the `--group` flag is not used.
When a default group exists *all targets should belong to at least one group*;
otherwise the configuration will become invalid and an error will be displayed
when running any command.

If no group is provided, and no `default-group` is defined, the operations
will be performed against targets without group definitions.

### Login
Requests a Kubernetes access token for each of the configured targets
and creates the kubeconfig's cluster, user and context elements for them.

```
$ osprey user login
user: someone
password: ***
Logged in to local.cluster
```
* Note: When using a cloud identity provider, a link to the respective online
login form will be shown in the terminal. The user must click on this link and
follow the login steps.

It will generate the kubeconfig file creating a `cluster` and `user` entry
per osprey target and one context with the `target` name and as many extra
contexts as `aliases` have been specified.

When specifying the `--group` flag, the operations will apply to the targets
belonging to the specified group. If targeting a group (provided or default)
the output will include the name of the group.
```
$ osprey user login --group foobar
user: someone
password: ***

Logging in to group 'foobar'

Logged in to foo.cluster | foo
Logged in to bar.cluster
```

At login, aliases are displayed after the pipes (i.e `| foo`)

### User
Displays information about the currently logged-in user (it shows the details
even if the token has already expired).
It contains the email of the logged-in user and the list of LDAP membership
groups the user is a part of. The latter come from the claims in the
user's token.

```
$ osprey user --group foobar
foo.cluster: someone@email.com [membership A, membership B]
bar.cluster: someone@email.com [membership C]
```

If no user is logged in, osprey displays `none` instead of the user details.

### Logout
Removes the token for the currently logged-in user for every configured
target.

```
$ osprey user logout --group foobar
Logged out from foo.cluster
Logged out from bar.cluster
```

If no user is logged in the command is a no-op.

### Config
This command is currently a no-op, used only to group the commands related
to the osprey configuration.

### Targets
Displays the list of defined targets within the client configuration.
It allows displaying the list of targets per group and to target a specific
group via flags.

```
$ osprey config targets --by-groups
Configured targets:
* <ungrouped>
    local.cluster
  bar
    bar.cluster
  foo
    foo.cluster | foo
  foobar
    bar.cluster
    foo.cluster | foo
```

This command will display targets that do not belong to any group, if there
are any, under the special group `<ungrouped>`.

If the configuration specifies a default group, it will be highlighted
with a `*` before its name, e.g. `* foobar`. If no default group is defined
the special `<ungrouped>` grouping will be highlighted.

#### Groups
The targets command flag `--list-groups` is useful to display only the
list of existing groups within the configuration, without any target
information.
```
$ osprey config targets --list-groups
Configured groups:
* <ungrouped>
  bar
  foo
  foobar
```

## Client configuration
The client installation script gets the configuration supported by the
installed version.

The client uses a YAML configuration file. Its recommended location is:
`$HOME/.osprey/config`. Its contents are as follows:
```yaml
# Optional path to the kubeconfig file to load/update when loging in.
# Uses kubectl defaults if absent ($HOME/.kube/config).
# kubeconfig: /home/jdoe/.kube/config

# Optional group name to be the default for all commands that accept it.
# When this value is defined, all targets must define at least one group.
# default-group: my-group

# Named map of supported providers (currently `osprey` and `azure`)
providers:
  osprey:
    # CA cert to use for HTTPS connections to Osprey.
    # Uses system's CA certs if absent.
    # certificate-authority: /tmp/osprey-238319279/cluster_ca.crt

    # Alternatively, a Base64-encoded PEM format certificate.
    # This will override certificate-authority if specified.
    # certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk5vdCB2YWxpZAotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==

    # Named map of target Osprey servers to contact for access-tokens
    targets:
      # Target Osprey's environment name.
      # Used for the name of the cluster, context, and users generated
      foo.cluster:
        # hostname:port of the target osprey server
        server: https://osprey.foo.cluster

        #  list of names to generate additional contexts against the target.
        aliases: [foo.alias]

        #  list of names that can be used to logically group different Osprey servers.
        groups: [foo]

        # CA cert to use for HTTPS connections to Osprey.
        # Uses system's CA certs if absent.
        # certificate-authority: /tmp/osprey-238319279/cluster_ca.crt

        # Alternatively, a Base64-encoded PEM format certificate.
        # This will override certificate-authority if specified.
        # certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk5vdCB2YWxpZAotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==

  # Authenticating against Azure AD
  azure:
    # These settings are required when authenticating against Azure
    tenant-id: your-azure-tenant-id
    server-application-id: azure-ad-server-application-id
    client-id: azure-ad-client-id
    client-secret: azure-ad-client-secret

    # List of scopes to request as part of the request. This should be an Azure link to the API exposed on the server application
    scopes:
      - "api://azure-tenant-id/Kubernetes.API.All"

    # This is required for the browser-based authentication flow. The port is configurable, but it must conform to
    # the format: http://localhost:<port>/auth/callback
    redirect-uri: http://localhost:65525/auth/callback
    targets:
      foo.cluster:
        server: http://osprey.foo.cluster
        # If "use-gke-clientconfig" is specified (default false) Osprey will fetch the API server URL and its
        # CA cert from the GKE-specific ClientConfig resource in kube-public. This resource is created automatically
        # by GKE when you enable to OIDC Identity Service. The "api-server" config element is also required.
        # Usually "api-server" would be set to the public API server endpoint; the fetched API server URL will be
        # the internal load balancer that proxies requests through the OIDC service.
        # use-gke-clientconfig: true
        #
        # If "skip-tls-verify" is specified (default false) Osprey will skip TLS verification when attempting
        # to make the connection to the specified server.  This can be used in conjunction with `server` or `api-server`.
        # skip-tls-verify: true
        #
        # If api-server is specified (default ""), Osprey will fetch the CA cert from the API server itself.
        # Overrides "server". A ConfigMap in kube-public called kube-root-ca.crt should be made accessible
        # to the system:anonymous group. This ConfigMap is created automatically with the Kubernetes feature
        # gate RootCAConfigMap which was alpha in Kubernetes v1.13 and became enabled by default in v1.20+
        # api-server: http://apiserver.foo.cluster
        aliases: [foo.alias]
        groups: [foo]
```

The name of the configured targets will be used to name the managed clusters,
contexts, and user. They can be setup as desired. Use the `aliases` property
of the targets to create alias contexts in the kubeconfig.

The previous configuration will result in the following `kubeconfig` file for the
user `jdoe`:

```
$ osprey user login --ospreyconfig /tmp/osprey-238319279/.osprey/config
```

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: YUhSMGNITTZMeTloY0dselpYSjJaWEl1YzJGdVpHSnZlQzVqYjNOdGFXTXVjMnQ1
    server: https://apiserver.foo.cluster
  name: foo.cluster
contexts:
- context:
    cluster: foo.cluster
    user: foo.cluster
  name: foo.cluster
- context:
    cluster: foo.cluster
    user: foo.cluster
  name: foo.alias
current-context: ""
preferences: {}
users:
- name: foo.cluster
  user:
    auth-provider:
      config:
        client-id: oidc_client_id
        client-secret: oidc_client_secret
        id-token: jdoe_Token
        idp-certificate-authority-data: aHR0cHM6Ly9kZXguc2FuZGJveC5jb3NtaWMuc2t5
        idp-issuer-url: https://dex.foo.cluster
      name: oidc
```

The client will create/update one instance of `cluster`, `context`, and `user`
in the `kubeconfig` file per `target` in the `ospreyconfig` file. We use
`client-go`'s config api to manipulate the `kubeconfig`.

If previous contexts exist in the kubectl config file, they will get
updated/overridden when performing a login. It overrides values by `name`
(e.g. `cluster.name`, `context.name`, `user.name`).
It is recommended that the first time using the Osprey for a specific cluster
old values are removed, to keep the config clean.

The names of clusters, user and context will use the value defined in the
Osprey config.

# Server

The Osprey server can be started in two different ways:
- `osprey serve cluster-info`
- `osprey serve auth`

### `osprey serve cluster-info`
Starts an instance of the Osprey serve that will create a webserver that is capable of returning cluster information. In
this mode, authentication is disabled. This endpoint is used for service discovery for an osprey target.

This endpoint (`/cluster-info`) will return the API server URL and the CA for the API server.

In this mode, the required flags are:

- `apiServerCA`, the path to the API server CA (defaults to `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt`) which
is the default location of the CA when running inside a Kubernetes cluster.
- `apiServerURL`, the API server URL to return to the Osprey client

Note that since v2.5.0 Osprey client can fetch the CA cert directly from the API server without needing a
deployment of Osprey server.

### `osprey serve auth`
Starts an instance of the osprey server that will listen for authentication
requests. The configuration is done through the commands flags. The Osprey service will receive the user's credentials
and forward them to the OIDC provider (Dex) for authentication. On success it will return the token generated by the
provider along with additional information about the cluster so that the client can generate the kubectl config file.
```
$ osprey serve auth --help
```

When Osprey is being used for authentication, the following flags require to be
the same across the specified components:

- `environment`, id of the cluster to be used as a client id
  - Dex: registered client `id` (managed via the Dex api or `staticClients`)
  - Kubernetes API server: `oidc-client-id` flag
- `secret`, token to be shared between Dex and Osprey
  - Dex: registered client `secret` (managed via the Dex api or `staticClients`)
- `redirectURL`, Osprey's callback url
  - Dex: registered client `redirectURIs` (managed via the Dex api or `staticClients`)
- `issuerURL`, Dex's URL
  - Dex: `issuer` value
  - Kubernetes API server: `oidc-issuer-url` flag
- `issuerCA`, Dex's CA certificate path
  - Kubernetes API server: `oidc-ca-file` flag

The following diagram depicts the authentication flow from the moment the
Osprey client requests a token.

```
                                       +------------------------+
                                       |                        |
                                       |      +----------------------------+
                                       |      |                 |          |
+------------------+                   |  +---v--------------+  |          |
|                  | 1./access-token   |  |                  |  |          |
|  Osprey Client   +---------------------->   Osprey Server  +-----+       |
|                  |                   |  |                  |  |  |       |
+------------------+                   |  +--+--------+------+  |  |       |
                                       |     |        |         |  |       |
                                       |     |        |         |  |       |
                                       |     | 2.     | 3.      |  |       |
                                       |     |/auth   |/login   |  |6. code|exchange
                                       |     |        |         |  |       |
                                       |     |        |         |  |       |
+------------------+                   |  +--v--------v------+  |  |       |
|                  |                   |  |                  |  |  |       |
|       LDAP       | 4. authenticate   |  |       Dex        <-----+       |
|                  <----------------------+                  +-------------+
+------------------+                   |  +------------------+  |  5. /callback
                                       |                        |
                                       |      Environment       |
                                       +------------------------+
```

After the user enters their credentials through the `Osprey Client`:
1. An HTTPS call is made to an `Osprey Server` per environment configured.
2. Per environment:
   1. The `Osprey Server` will make an authentication request to `Dex` which
      will return an authentication url to use and a request ID.
   2. The `Osprey Server` will post the user credentials using the auth request
      details.
   3. `Dex` will call `LDAP` to validate the user.
   4. Upon a successful validation, `Dex` will redirect the request to the
      `Osprey Server`'s callback url, with a generated code and the request ID.
   5. The `Osprey Server` will exchange the code with `Dex` to get the final
      token that is then returned to the client.
   6. The `Osprey Client` updates the `kubeconfig` file with the updated token.

### TLS
Because the Osprey client sends the users credentials to the server, the
communication must always be done securely. The Osprey server has to run
using HTTPS, so a certificate and a key must be generated and provided at
startup.
The client must be configured with the CA used to sign the certificate in
order to be able to communicate with the server.

A script to generate a test self-signed certificate, key and CA can be found
in the [examples](examples/local/generate-certs.sh)

### Dex templates and static content
By default Dex searches for web resources in a `web` folder located in the
same directory where the server is started. This location can be overridden
in Dex's configuration:

```yaml
...
frontend:
  dir: /path/to/the/templates
  theme: osprey
...
```

Dex also requires a `web/static` folder and a `web/themes/<theme>` folder
for static content. Osprey does not require any of these, but the folders
are required to be there, even if empty.

Because the authentication flow does not involve the user, the data exchanged
between Dex and Osprey must be in `json` so the `html` templates need to be
customized.

A folder with the required configuration for Osprey can be taken from
[out test setup](e2e/dextest/web). The only theme is `osprey` and it is
empty. All the templates file are required to be present, but not all of
them are used in the authentication flow.

### Identity Provider
Osprey doesn't currently support Dex using multiple Identity Providers
as the user would be required to select one of them (`login.html`) before
proceeding to the authentication request.

Therefore currently **only one** Identity Provider can be configured.

### Token expiry and Refresh token
Dex allows for configuration of the token expiry, and it also provides
a refresh token, so that a client can request a new token without the need
of user interaction.

The current usage of Osprey is such that it was decided to discard the
refresh token, to prevent a compromised token to be active for more than
a configured amount of time. If the need arises, this could be reintroduced
and enabled/disabled by configuration.

### API server
The Kubernetes API server needs to [enable the OIDC Authentication](https://kubernetes.io/docs/admin/authentication/#configuring-the-api-server)
in order for the kubectl requests to be authenticated and then authorised.

Some of those flags have been mentioned in the [configuration](#osprey-serve-auth).

# Examples

[Download and install](#binaries) the Osprey binaries so that the client
can be used for the examples.

## Kubernetes
A set of examples resources has been provided to create the required resources
to deploy Dex and Osprey to a Kubernetes cluster.
The templates can be found in `examples/kubernetes`.

1. Provide the required properties in `examples/kubernetes/kubernetes.properties`:
   - `node`, the script uses a NodePort service, so in order to configure the
     Osprey and Dex to talk to each other, a node IP from the target cluster
     must be provided.
     A list of IPs to chose from can be obtained via:
     ```
     kubectl --context <context> get nodes -o template --template='{{range.items}}{{range.status.addresses}}{{if eq .type "InternalIP"}} {{.address}}:{{end}}{{end}}{{end}}' | tr ":" "\n"
     ```
   - `context`, the script uses kubectl to apply the resources and for this it
     needs a context to target.
   - `ospreyNodePort`, `dexNodePort`, `dexTelemetryNodePort`, the ports
     where Osprey and Dex (service and metrics) will be available across
     the cluster. A default value is provided, but if the ports are already
     in use, they must be changed.
   - `ospreyImage`, if you want to try a different version for the server.

2. Run the shell script to render the templates and to deploy the resources
   to the specified cluster.
   ```
   examples/kubernetes/deploy-all.sh </full/path/to/runtime/dir>
   ```
   To create an Osprey server that serves `/cluster-info` only, set `ospreyAuthenticationDisabled=true` in the properties
   file.
3. Use the Osprey client
   ```
   osprey --ospreyconfig </full/path/to/runtime/dir/>osprey/ospreyconfig --help
   ```

More properties are available to customize the resources at will.

## Local docker containers
Although the Osprey solution is intended to be run in a Kubernetes cluster,
with the OIDC Authentication enabled, it is possible to have a local instance
of Osprey and Dex to try out and validate a specific configuration.

A set of scripts have been provided to start an end to end run of a user
logging in, checking details and logging out.

From the root of the project:
```
$ mkdir /tmp/osprey_local
$ examples/local/end-to-end.sh /tmp/osprey_local
```
The [end-to-end.sh](examples/local/end-to-end.sh) script will:
1. Start a Dex server ([start-dex.sh](examples/local/start-dex.sh))
2. Start an Osprey server ([start-osprey.sh](examples/local/start-osprey.sh))
3. Execute the `osprey user login` command. It will request credentials,
   use user/pass: `jane@foo.cluster`/`doe`, `john@foo.cluster`/`doe`
4. Execute the `osprey user` command
5. Execute the `osprey user logout` command
6. Execute the `osprey user` command
7. Shutdown Osprey and Dex

You can also start Dex and Osprey manually with the scripts and play with
the Osprey client yourself.

The scripts use templates for the [Dex configuration](examples/local/dex/config.template.yml)
and the [Osprey client configuration](examples/local/osprey/ospreyconfig.template).
The scripts load a [properties file](examples/local/local.properties) to
render the templates.


# Development

First install the required dependencies:

* `make setup` - installs required golang binaries
* `slapd`, usually part of the `openldap` package - needed for end-to-end tests

Then run the tests using make:

```sh
$ make
```

## Package structure

* `/cmd` contains the cobra commands to start a server or a client.
* `/client` contains code for the osprey cli client.
* `/server` contains code for the osprey server.
* `/common` contains code common to both client and server.
* `/e2e` contains the end-to-end tests, and test utils for dependencies.
* `/examples` contains scripts to start Dex and Osprey in a Kubernetes clusters
  or locally.
* `vendor` contains the dependencies of the project.

## Server and client

We use [Cobra](https://github.com/spf13/cobra), to generate the client and server commands.

### E2E tests
The e2e tests are executed against local Dex and LDAP servers.

The setup is as follows:

Osprey Client (1) -> (*) Osprey Server (1) -> (1) Dex (*) -> (1) LDAP

Each pair of `osprey server`-`Dex` represents an environment (cluster) setup.
One `osprey client` contacts as many `osprey-servers` as configured in the
test setup.
Each `osprey server` will talk to only one `Dex` instance located in the same
environment.
All `Dex` instances from the different environments will talk to the single
`LDAP` instance.

For cloud end-to-end tests, a mocked OIDC server is created and used to authenticate with.

## HTTPS/ProtocolBuffers

Given that aws ELBs do not support HTTP/2 osprey needs to run over HTTP.
We still use ProtocolBuffers for the requests and responses between osprey
and its client.

*Any changes made to the proto files should be backwards compatible.* This guarantees older clients can continue
to work against Osprey, and we don't need to worry about updates to older clients.

To update, update `common/pb/osprey.proto` then run protoc.

```
$ make proto
```

Check in the `osprey.pb.go` file afterwards.

## Azure Active Directory setup

The Azure AD Application setup requires two applications to be created. One for the Kubernetes API servers to use, and
one for the Osprey client to use. The Osprey client is then configured to request access on behalf of the Kubernetes
OIDC provider.

### Create Osprey Kubernetes Application
1. Visit https://portal.azure.com/#blade/Microsoft_AAD_IAM/ActiveDirectoryMenuBlade/Overview and log in using your
   organisations credentials.
2. Select 'App Registrations' from the side-bar and click '+ New registration' on the top menu bar.
3. Create an application with the following details:
   - Name: "Osprey - Kubernetes API Server"
   - Supported account types: "Accounts in this organizational directory only"
4. Select 'API permissions' from the side-bar and click '+ Add a permission'
   Add the following permissions:
     - Microsoft Graph -> Delegated permissions -> Enable access to "email", "openid" and "profile"
     - Click 'Add permissions' to save.
5. Select 'Expose an API' from the side-bar and click '+ Add a scope'
6. Create a scope with an appropriate/descriptive name. e.g. `Kubernetes.API.All`. The details in this form are what are
   shown to users when they first authorize the application to log in on their behalf.
7. Select 'Manifest' from the side-bar and find the field `groupMembershipClaims` in the JSON. Change this so that its
   value is `"groupMembershipClaims": "All",` and not `"groupMembershipClaims": null,`
8. The *server* client-id  is the Object ID of this application. This can be found in the Overview panel.

### Create Osprey Client Application
1. Visit https://portal.azure.com/#blade/Microsoft_AAD_IAM/ActiveDirectoryMenuBlade/Overview and log in using your
   organisations credentials.
2. Select 'App Registrations' from the side-bar and click '+ New registration' on the top menu bar.
3. Create an application with the following details:
   - Name: "Osprey - Client"
   - Supported account types: "Accounts in this organizational directory only"
   - RedirectURI:
     - Type: Web
       RedirectURI: This is a redirect URI that must be configured to match in both the Azure application config and the
       Osprey config. It has to be in the `http://localhost:<port>/<path>` format. This will be the port that Osprey
       client opens up a webserver on, to listen to callbacks from the login page. We use `http://localhost:65525/auth/callback` in the
       example configuration.
4. Select 'API permissions' from the side-bar and click '+ Add a permission'
   Add the following permissions:
     - Microsoft Graph -> Delegated permissions -> Enable access to "openid"
     - Click 'Add permissions' to save.
5. Click '+ Add a permission' and select 'My APIs' from the top of the pop-out menu.
     - Select the "Osprey - Kubernetes API Server"
     - Click 'Add permissions' to save.
6. Select 'Certificates & secrets' from the side-bar and click '+ New client secret'
   - Choose an expiry for this secret. When a token expires, the osprey client config must be updated to include this as
     the 'client-secret'. Copy this secret as soon as it is created, as it will be hidden when you leave the azure pane.
7. The *osprey* client-id  is the Object ID of this application. This can be found in the Overview panel.


The client ID and secrets generated in this section are used to fill out the Osprey config file.
```yaml
providers:
  azure:
    tenant-id: your-tenant-id
    server-application-id: api://SERVER-APPLICATION-ID   # Application ID of the "Osprey - Kubernetes APIserver"
    client-id: azure-application-client-id               # Client ID for the "Osprey - Client" application
    client-secret: azure-application-client-secret       # Client Secret for the "Osprey - Client" application
    scopes:
      # This must be in the format "api://" due to non-interactive logins appending this to the audience in the JWT.
      - "api://SERVER-APPLICATION-ID/Kubernetes.API.All"
    redirect-uri: http://localhost:65525/auth/callback   # Redirect URI configured for the "Osprey - Client" application
```

Kubernetes API server flags:
```yaml
- --oidc-issuer-url=https://sts.windows.net/<tenant-id>/
- --oidc-client-id=api://9bd903fd-f8df-4390-9a45-ab2fa28673b4
- --oidc-username-claim=unique_name
- --oidc-groups-claim=groups
```

## Dependency management

Dependencies are managed with [Go modules](https://github.com/golang/go/wiki/Modules).
Run `go mod download` to download all dependencies.

Make sure any Kubernetes dependencies are compatible with the `kubernetes-1.8.5`

# Releasing

Tag the commit in `master` using an annotated tag and push it to release it.
Only maintainers can do this.

Osprey gets released to:
- [Docker-Hub](https://hub.docker.com/r/skycirrus/osprey/) as an alpine based docker image.

# Code guidelines

* Follow [Effective Go](https://go.dev/doc/effective_go).
