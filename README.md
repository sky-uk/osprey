# Osprey

Client and service for providing access to Kubernetes clusters.

The client provides a `user login` command which will request a `username`
and a `password` and forward them to the service. The service will forward
the credentials to an OpenId Connect Provider (OIDC) to authenticate the
user and will return a JWT token with the user details. The token along
with some additional cluster information will be used to generate the
`kubectl` configuration to be used to access Kubernetes clusters.

The implementation relies in one specific configuration detail of the OIDC
provider `SkipApprovalScreen : true` which eliminates the intermediate step
requiring a client to explicitly approve the requested grants before the
token is provided. If the target provider does not support this feature,
additional work is required to handle that approval.

Although the OIDC provider may support multiple providers, the current
implementation of the Osprey service only supports the flow for one provider.

## Quick links
- [Installation](#installation)
- [Client](#client)
- [Server](#server)
- [Examples](#examples)

## Installation

Osprey is currently supported in linux, mac OS and windows and it can be
installed as a standalone executable, or it can be run as a container (only
linux).
The docker container is aimed to be used for the server side, while the
binaries main use are the client commands.

### Binaries

Osprey's executable binaries can be downloaded from our [Bintray repository](https://dl.bintray.com/sky-uk/oss-generic/osprey).

To install a specific version replace `latest` with the release version.

**Linux and Mac OS**
```
  curl -L https://dl.bintray.com/sky-uk/oss-generic/osprey/latest/osprey-latest_linux_amd64.tar.gz -o osprey.tar.gz

  tar -xvf osprey.tar.gz -C $HOME/.local/bin
  chmod +x $HOME/.local/bin/osprey
```
To install for Mac OS replace `linux` for `darwin`

**Windows**
```
  mkdir c:\osprey
  Invoke-WebRequest -method Get -uri https://dl.bintray.com/sky-uk/oss-generic/osprey/latest/osprey-latest_windows_amd64.tar.gz -OutFile c:\osprey\osprey.zip

  Unzip c:\osprey\osprey.zip c:\osprey

  $env:Path = "c:\osprey;" + $env:Path
  [Environment]::SetEnvironmentVariable( "Path", $env:Path, [System.EnvironmentVariableTarget]::Machine )
```

### Docker

The docker image is based on alpine. It can be pulled from our [Docker Hub repository](https://hub.docker.com/r/skycirrus/osprey/)

To pull a specific version replace `latest` with the release version.

```
   docker pull skycirrus/osprey:latest
```

# Client

The `osprey client` will request the user credentials and generate a
kubeconfig file based on the contents of its [configuration](#client-configuration).

## Client usage
- [login](#login)
- [logout](#logout)
- [user](#user)

With a [configuration](#client-configuration) file like:
```
targets:
  foo.cluster:
    server: https://osprey.foo.cluster
    alias: [foo]
  bar.cluster:
    server: https://osprey.bar.cluster
```

### Login
Requests a kubernetes access token for each of the configured targets and
and creates the kubeconfig's cluster, user and context elements for them.

```
$ osprey user login
user: someone
password: ***
Logged in to foo.cluster | foo
Logged in to bar.cluster
```

It will generate the kubeconfig file creating a `cluster` and `user` entry
per osprey target and one context with the `target` name and as many extra
contexts as `aliases` have been specified.

At login, aliases are displayed after the pipes (i.e `| foo`)

### User
Displays information about the currently logged in user (it shows the details
even if the token has already expired)

```
$ osprey user
foo.cluster: someone@email.com [group1, group2]
bar.cluster: someone@email.com [group1, group2]
```

If no user is logged in, osprey displays `none` instead of the user details.

### Logout
Removes the token for the currently logged in user for every configured
target.

```
$ osprey user logout
Logged out from foo.cluster
Logged out from bar.cluster
```

If no user is logged in the command is a no-op.

## Client configuration
The client installation script gets the configuration supported by the
installed version.

The client uses a yaml configuration file. It's recommended location is:
`$HOME/.osprey/config`. Its contents are as follow:
```
# Optional path to the kubeconfig file to load/update when loging in.
# Uses kubectl defaults if absent ($HOME/.kube/config).
# kubeconfig: /home/jdoe/.kube/config

# Mandatory for windows, optional for unix systems.
# CA cert to use for HTTPS connections to osprey.
# Uses system's CA certs if absent (only in unix systems).
# certificate-authority: /tmp/osprey-238319279/cluster_ca.crt

# Alternatively, base64-encoded PEM format certificate.
# This will override certificate-authority if specified.
# Same caveat for Windows systems applies.
# certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk5vdCB2YWxpZAotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==

# Named map of target osprey servers to contact for access-tokens
targets:
  # Target osprey's environment name.
  # Used for the name of the cluster, context, and users generated
  foo.cluster:
    # hostname:port of the target osprey server
    server: https://osprey.foo.cluster

    #  list of names to generate aditional contexts against the target.
    aliases: [foo.alias]

    # Mandatory for windows, optional for unix systems.
    # CA cert to use for HTTPS connections to osprey.
    # Uses system's CA certs if absent (only in unix systems).
    # certificate-authority: /tmp/osprey-238319279/cluster_ca.crt

    # Alternatively, base64-encoded PEM format certificate.
    # This will override certificate-authority if specified.
    # Same caveat for Windows systems applies.
    # certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk5vdCB2YWxpZAotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==

```

The name of the configured targets will be used to name the managed clusters,
contexts, and user. They can be setup as desired. Use the `aliases` property
of the targets to create alias contexts in the kubeconfig.

The previous configuration will result in the following `kubeconfig` file for the
user `jdoe`:

`osprey user login --ospreyconfig /tmp/osprey-238319279/.osprey/config`

```
apiVersion: v1
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
kind: Config
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
updated/overriden when performing a login. It overrides values by `name`
(e.g. cluster.name, context.name, user.name).
It is recommended that the first time using the osprey for a specific cluster
old values are removed, to keep the config clean.

The names of clusters, user and context will use the value defined in the
osprey config.

# Server

The Osprey service will receive the user's credentials and forward them to
the OIDC provider (Dex) for authentication. On success it will return the
token generated by the provider along with additional information about the
cluster so that the client can generate the kubectl config file.

## Server usage

### Serve
Starts an instance of the osprey server that will listen for authentication
requests. The configuration is done through the commands flags.
```
    osprey serve --help
```

## Server configuration

The following flags require to be the same across the specified components:

- `environment`, id of the cluster to be used as a client id
  - Dex: registered client `id` (managed via the Dex api or `staticClients`)
  - Kubernetes apiserver: `oidc-client-id` flag
- `secret`, token to be shared between Dex and Osprey
  - Dex: registered client `secret` (managed via the Dex api or `staticClients`)
- `redirectURL`, Osprey's callback url
  - Dex: registered client `redirectURIs` (managed via the Dex api or `staticClients`)
- `issuerURL`, Dex's URL
  - Dex: `issuer` value
  - Kubernetes apiserver: `oidc-issuer-url` flag
- `issuerCA`, Dex's CA certificate path
  - Kubernetes apiserver: `oidc-ca-file` flag


The following diagram depicts the authentication flow from the moment the
osprey client requests a token.

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

After the user enters its credentials through the `Osprey Client`:
1. An HTTPS call is made to an `Osprey Server` per environment configured.
2. Per environment:
   1. The `Osprey Server` will make an authentication request to `Dex` which will
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

A script to generate a test self signed certificate, key and CA can be found
in the [examples](examples/local/generate-certs.sh)

### Dex templates and static content
By default Dex searches for web resources in a `web` folder located in the
same directory where the server is started. This location can be overridden
in Dex's configuration:

```
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

The current usage of osprey is such that it was decided to discard the
refresh token, to prevent a compromised token to be active for more than
a configured amount of time. If the need arises, this could be reintroduced
and enabled/disabled by configuration.

### Apiserver
The Kubernetes apiserver needs to [enable the OIDC Authentication](https://kubernetes.io/docs/admin/authentication/#configuring-the-api-server)
in order for the kubectl requests to be authenticated and then authorised.

Some of those flags have been mentioned in the [configuraion](#server-configuration).

# Examples

[Download and install](#binaries) the Osprey binaries so that the client
can be used for the examples.

## Kubernetes
A set of examples resources has been provided to create the required resources
to deploy Dex and Osprey to a kubernetes cluster.
The templates can be found in `examples/kubernetes`.

1. Provide the required properties in `examples/kubernetes/kubernetes.properties`:
   - `node`, the script uses a NodePort service, so in order to configure the
     osprey and dex to talk to each other, a node ip from the target cluster
     must be provided.
     A list of ips to chose from can be obtained via:
     ```
     kubectl --context <context> get nodes -o template --template='{{range.items}}{{range.status.addresses}}{{if eq .type "InternalIP"}}  {{.address}}:{{end}}{{end}}{{end}}' | tr ":" "\n"
     ```
   - `context`, the script uses kubectl to apply the resources and for this it
     needs a context to target.
   - `ospreyNodePort`, `dexNodePort`, `dexTelemetryNodePort`, the ports
     where Osprey and Dex (service and metrics) will be available across
     the cluster. A default value is provided, but if the ports are already
     in use, they must be changed.
   - `ospreyImage`, if you want to try a different version for the server.

2. Run the shell script to render the templates and to to deploy the resources
   to the specified cluster.
   ```
   examples/kubernetes/deploy-all.sh </full/path/to/runtime/dir>
   ```
3. Use the osprey client
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
   mkdir /tmp/osprey_local
   examples/local/end-to-end.sh /tmp/osprey_local
```
The [end-to-end.sh](examples/local/end-to-end.sh) script will:
1. Start a Dex server ([start-dex.sh](examples/local/start-dex.sh))
2. Start an Osprey server ([start-osprey.sh](examples/local/start-osprey.sh))
3. Execute the `osprey user login` command. It will request credentials,
   use user/pass: "jane@foo.cluster/doe", "john@foo.cluster/doe"
4. Execute the `osprey user` command
5. Execute the `osprey user logout` command
6. Execute the `osprey user` command
7. Shutdown Osprey and Dex

You can also start Dex and Osprey manually with the scripts and play with
the osprey client yourself.

The scripts use templates for the [Dex configuration](examples/local/dex/config.template.yml)
and the [Osprey client configuration](examples/local/osprey/ospreyconfig.template).
The scripts load a [properties file](examples/local/local.properties) to
render the templates.


# Development
To setup the environment with the required dependencies:
```
    ./make setup
```
To build and run all tests:

```
    ./make
```

## Package structure

* `/cmd` contains the cobra commands to start a server or a client.
* `/client` contains code for the osprey cli client.
* `/server` contains code for the osprey server.
* `/common` contains code common to both client and server.
* `/e2e` contains the end to end tests, and test utils for dependencies.
* `/examples` contains scripts to start Dex and Osprey in a Kubernetes clusters
  or locally.
* `vendor` contains the dependencies of the project.

## Server and client

We use [cobra](https://github.com/spf13/cobra), to generate the client and server commands.

### E2E tests
The e2e tests are executed against local Dex and ldap servers.

The setup is as follows:

Osprey Client (1) -> (*) Osprey Server (1) -> (1) Dex (*) -> (1) LDAP

Each pair of `osprey server`-`Dex` represents an environment (cluster) setup.
One `osprey client` contacts as many `osprey-servers` as configured in the
test setup.
Each `osprey server` will talk to only one `Dex` instance located in the same
environment.
All `Dex` instances from the different environments will talk to the single
`LDAP` instance.

## HTTPS/ProtocolBuffers

Given that aws ELB's do not support HTTP/2 osprey needs to run over HTTP.
We still use ProtocolBuffers for the requests and responses between osprey
and its client.

*Any changes made to the proto files should be backwards compatible.* This guarantees older clients can continue
to work against osprey, and we don't need to worry about updates to older clients.

To update, update `common/pb/osprey.proto` then run protoc.

    make proto

Check in the `osprey.pb.go` file afterwards.

## Dependency management

Dependencies are managed with [dep](https://golang.github.io/dep/).
Run `dep ensure` to keep your vendor folder up to date after a pull.

Make sure any kubernetes dependencies are compatible with the `kubernetes-1.8.5`

# Releasing

Tag the commit in master and push it to release it. Only maintainers can do this.

Osprey gets released to:
- [Bintray](https://bintray.com/sky-uk/oss-generic/osprey) as binaries for the supported platforms.
- [Docker-Hub](https://hub.docker.com/r/skycirrus/osprey/) as an alpine based docker image.

# Code guidelines

* Follow Effective Go.
