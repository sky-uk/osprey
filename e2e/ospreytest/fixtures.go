package ospreytest

import (
	"fmt"

	"io/ioutil"

	"github.com/SermoDigital/jose/jws"
	"github.com/SermoDigital/jose/jwt"
	"github.com/sky-uk/osprey/server/osprey"
	clientgo "k8s.io/client-go/tools/clientcmd/api"
)

const targetNamePrefix = "kubectl."
const targetAliasPrefix = "alias."

// ToKubeconfigCluster returns a *Cluster representation of the TestOsprey instance.
func (o *TestOsprey) ToKubeconfigCluster() *clientgo.Cluster {
	apiServer := fmt.Sprintf("https://apiserver.%s.cluster", o.Environment)
	caData, _ := ioutil.ReadFile(o.APIServerCA)

	expectedCluster := clientgo.NewCluster()
	expectedCluster.Server = apiServer
	expectedCluster.CertificateAuthorityData = caData
	return expectedCluster
}

// ToKubeconfigUserWithoutToken returns an *AuthInfo representation, with an empty id-token, of the TestOsprey instance.
func (o *TestOsprey) ToKubeconfigUserWithoutToken() *clientgo.AuthInfo {
	caData, _ := osprey.ReadAndEncodeFile(o.IssuerCA)
	authInfo := clientgo.NewAuthInfo()
	authProviderConfig := make(map[string]string)
	authProviderConfig["client-id"] = o.Environment
	authProviderConfig["client-secret"] = o.Secret
	authProviderConfig["id-token"] = ""
	authProviderConfig["idp-certificate-authority-data"] = caData
	authProviderConfig["idp-issuer-url"] = o.IssuerURL
	authInfo.ImpersonateUserExtra = nil
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

// OspreyconfigTargetName returns the ospreyconfig target's name for the TestOsprey instance.
func (o *TestOsprey) OspreyconfigTargetName() string {
	return fmt.Sprintf("%s%s", targetNamePrefix, o.Environment)
}

// OspreyconfigAliasName returns the ospreyconfig target's alias for the TestOsprey instance.
func (o *TestOsprey) OspreyconfigAliasName() string {
	return fmt.Sprintf("%s%s", targetAliasPrefix, o.OspreyconfigTargetName())
}

// ToKubeconfigContext returns a *Context representation of the TestOsprey instance.
func (o *TestOsprey) ToKubeconfigContext() *clientgo.Context {
	targetName := o.OspreyconfigTargetName()

	kubeconfigCtx := clientgo.NewContext()
	kubeconfigCtx.Cluster = targetName
	kubeconfigCtx.AuthInfo = targetName

	return kubeconfigCtx
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

func extractClaims(token jwt.JWT) (groups []string, err error) {
	claimedGroups := token.Claims().Get("groups")
	for _, group := range claimedGroups.([]interface{}) {
		groups = append(groups, group.(string))
	}
	return groups, err
}
