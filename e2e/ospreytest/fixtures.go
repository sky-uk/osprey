package ospreytest

import (
	"fmt"

	"io/ioutil"

	"net/http"

	"github.com/SermoDigital/jose/jws"
	"github.com/SermoDigital/jose/jwt"
	"github.com/sky-uk/osprey/common/web"
	"github.com/sky-uk/osprey/server/osprey"
	"k8s.io/client-go/tools/clientcmd"
	clientgo "k8s.io/client-go/tools/clientcmd/api"
)

const targetNamePrefix = "kubectl."
const targetAliasPrefix = "alias."

//AddCustomNamespaceToContexts adds a namespace to each context in the kubeconfig file
// the name of the namespace will be
func AddCustomNamespaceToContexts(namespaceSuffix, kubeconfig string, targetedOspreys []*TestOsprey) error {
	existingConfig, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return fmt.Errorf("unable to load kubeconfig file %s: %v", kubeconfig, err)
	}
	for _, target := range targetedOspreys {
		targetName := target.OspreyconfigTargetName()
		existingConfig.Contexts[targetName].Namespace = target.CustomTargetNamespace(namespaceSuffix)

		aliasTargetName := target.OspreyconfigAliasName()
		existingConfig.Contexts[aliasTargetName].Namespace = target.CustomAliasNamespace(namespaceSuffix)
	}
	err = clientcmd.WriteToFile(*existingConfig, kubeconfig)
	if err != nil {
		return fmt.Errorf("unable to write kubeconfig file %s: %v", kubeconfig, err)
	}
	return nil
}

// ToKubeconfigCluster returns a *Cluster representation of the TestOsprey instance.
func (o *TestOsprey) ToKubeconfigCluster(locationOfOrigin string) *clientgo.Cluster {
	apiServer := fmt.Sprintf("https://apiserver.%s.cluster", o.Environment)
	caData, _ := ioutil.ReadFile(o.APIServerCA)
	expectedCluster := clientgo.NewCluster()
	expectedCluster.LocationOfOrigin = locationOfOrigin
	expectedCluster.Server = apiServer
	expectedCluster.CertificateAuthorityData = caData
	return expectedCluster
}

// ToKubeconfigUserWithoutToken returns an *AuthInfo representation, with an empty id-token, of the TestOsprey instance.
func (o *TestOsprey) ToKubeconfigUserWithoutToken(locationOfOrigin string) *clientgo.AuthInfo {
	caData, _ := osprey.ReadAndEncodeFile(o.IssuerCA)
	authInfo := clientgo.NewAuthInfo()
	authProviderConfig := make(map[string]string)
	authProviderConfig["client-secret"] = o.Secret
	authProviderConfig["id-token"] = ""
	authProviderConfig["idp-certificate-authority-data"] = caData
	authProviderConfig["idp-issuer-url"] = o.IssuerURL
	authProviderConfig["access-token"] = ""
	authProviderConfig["client-id"] = o.Environment
	authInfo.ImpersonateUserExtra = nil
	authInfo.LocationOfOrigin = locationOfOrigin
	authInfo.AuthProvider = &clientgo.AuthProviderConfig{
		Name:   "oidc",
		Config: authProviderConfig,
	}
	return authInfo
}

// WithoutToken creates a DeepCopy() of authInfo without an id-token.
func WithoutToken(authInfo *clientgo.AuthInfo) *clientgo.AuthInfo {
	clone := authInfo.DeepCopy()
	clone.AuthProvider.Config["id-token"] = ""
	return clone
}

// OspreyTargetOutput returns the name and aliases output format for the environment.
func OspreyTargetOutput(environment string) string {
	return fmt.Sprintf("%s | %s", OspreyconfigTargetName(environment), OspreyconfigAliasName(environment))
}

// OspreyconfigTargetName returns the ospreyconfig target's name for the TestOsprey instance.
func (o *TestOsprey) OspreyconfigTargetName() string {
	return OspreyconfigTargetName(o.Environment)
}

// OspreyconfigTargetName returns the ospreyconfig target's name for the environment.
func OspreyconfigTargetName(environment string) string {
	return fmt.Sprintf("%s%s", targetNamePrefix, environment)
}

// OspreyconfigAliasName returns the ospreyconfig target's alias for the environment.
func OspreyconfigAliasName(environment string) string {
	return fmt.Sprintf("%s%s", targetAliasPrefix, OspreyconfigTargetName(environment))
}

// OspreyconfigAliasName returns the ospreyconfig target's alias for the TestOsprey instance.
func (o *TestOsprey) OspreyconfigAliasName() string {
	return OspreyconfigAliasName(o.Environment)
}

// ToKubeconfigContext returns a *Context representation of the TestOsprey instance.
func (o *TestOsprey) ToKubeconfigContext(locationOfOrigin string) *clientgo.Context {
	targetName := o.OspreyconfigTargetName()

	kubeconfigCtx := clientgo.NewContext()
	kubeconfigCtx.Cluster = targetName
	kubeconfigCtx.AuthInfo = targetName
	kubeconfigCtx.LocationOfOrigin = locationOfOrigin

	return kubeconfigCtx
}

// CustomTargetNamespace returns the name for a namespace appending the suffix to the osprey's target name.
func (o *TestOsprey) CustomTargetNamespace(suffix string) string {
	return o.OspreyconfigTargetName() + suffix
}

// CustomAliasNamespace returns the name for a namespace appending the suffix to the osprey's alias name.
func (o *TestOsprey) CustomAliasNamespace(suffix string) string {
	return o.OspreyconfigAliasName() + suffix
}

// ToGroupClaims returns the groups contained in the groups claim of the id-token for the authInfo.
// If no tokens exists it returns an empty slice.
func (o *TestOsprey) ToGroupClaims(authInfo *clientgo.AuthInfo) ([]string, error) {
	var groups []string
	tokenString := authInfo.AuthProvider.Config["id-token"]
	token, err := jws.ParseJWT([]byte(tokenString))
	if err != nil {
		return groups, err
	}
	return extractClaims(token)
}

// CallHealthcheck returns the current status of osprey's healthcheck as an http response and error
func (o *TestOsprey) CallHealthcheck() (*http.Response, error) {
	certData, _ := web.LoadTLSCert(o.CertFile)
	httpClient, err := web.NewTLSClient(false, certData)
	if err != nil {
		return nil, err
	}
	ospreyHealthCheckURL := fmt.Sprintf("%s/healthz", o.URL)
	req, err := http.NewRequest(http.MethodGet, ospreyHealthCheckURL, nil)
	if err != nil {
		return nil, err
	}
	return httpClient.Do(req)
}

// CreateCustom

// GetOspreysByGroup returns the ospreys matching by group or default group given the environmentGroups definition.
func GetOspreysByGroup(group, defaultGroup string, environmentGroups map[string][]string, ospreys []*TestOsprey) []*TestOsprey {
	var targetedOspreys []*TestOsprey
	actualGroup := group
	if actualGroup == "" {
		actualGroup = defaultGroup
	}
	for _, target := range ospreys {
		if groups, ok := environmentGroups[target.Environment]; ok {
			if len(groups) == 0 && actualGroup == "" {
				targetedOspreys = append(targetedOspreys, target)
			}
			for _, ospreyGroup := range groups {
				if actualGroup == ospreyGroup {
					targetedOspreys = append(targetedOspreys, target)
					break
				}
			}
		}
	}

	return targetedOspreys
}

func extractClaims(token jwt.JWT) (groups []string, err error) {
	claimedGroups := token.Claims().Get("groups")
	for _, group := range claimedGroups.([]interface{}) {
		groups = append(groups, group.(string))
	}
	return groups, err
}
