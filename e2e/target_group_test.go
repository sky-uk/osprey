package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/osprey/e2e/ospreytest"

	"github.com/sky-uk/osprey/client/kubeconfig"
	"github.com/sky-uk/osprey/e2e/clitest"
	clientgo "k8s.io/client-go/tools/clientcmd/api"
)

var _ = Describe("Target group", func() {
	var user, login, logout *clitest.CommandWrapper

	BeforeEach(func() {
		user = Client("user", ospreyconfigFlag)
		login = Client("user", "login", ospreyconfigFlag)
		logout = Client("user", "logout", ospreyconfigFlag)
	})

	It("logins successfully with good credentials", func() {
		login.LoginAndAssertSuccess("jane", "foo")
	})

	It("fails login with invalid credentials", func() {
		login.LoginAndAssertFailure("admin", "wrong")
	})

	It("creates a kubeconfig file on the specified location", func() {
		login.LoginAndAssertSuccess("jane", "foo")

		Expect(ospreyconfig.ConfigFile).To(BeAnExistingFile())
	})

	It("healthcheck should return ok", func() {
		for _, osprey := range ospreys {
			resp, err := osprey.CallHealthcheck()

			Expect(err).To(BeNil(), "could not call healthcheck")
			Expect(resp.StatusCode).To(Equal(200))
		}
	})

	It("logs in with certificate-authority-data", func() {
		caDataConfig, err := BuildCADataConfig(testDir, ospreys, true, "")
		Expect(err).To(BeNil(), "Creates the osprey config")
		caDataConfigFlag := "--ospreyconfig=" + caDataConfig.ConfigFile
		caDataLogin := Client("user", "login", caDataConfigFlag)

		caDataLogin.LoginAndAssertSuccess("jane", "foo")
	})

	It("logs in overriding certificate-authority with certificate-authority-data", func() {
		caDataConfig, err := BuildCADataConfig(testDir, ospreys, true, "/road/to/nowhere")
		Expect(err).To(BeNil(), "Creates the osprey config")
		caDataConfigFlag := "--ospreyconfig=" + caDataConfig.ConfigFile
		caDataLogin := Client("user", "login", caDataConfigFlag)

		caDataLogin.LoginAndAssertSuccess("jane", "foo")
	})

	Context("kubeconfig file", func() {
		var (
			generatedConfig *clientgo.Config
			username        string
		)

		BeforeEach(func() {
			username = "jane"
			login.LoginAndAssertSuccess(username, "foo")
			err := kubeconfig.LoadConfig(ospreyconfig.Kubeconfig)
			Expect(err).To(BeNil(), "successfully creates a kubeconfig")
			generatedConfig, err = kubeconfig.GetConfig()
			Expect(err).To(BeNil(), "successfully creates a kubeconfig")
		})

		It("contains a cluster per osprey", func() {
			for _, osprey := range ospreys {
				expectedCluster := osprey.ToKubeconfigCluster()
				expectedCluster.LocationOfOrigin = ospreyconfig.Kubeconfig
				target := osprey.OspreyconfigTargetName()
				Expect(generatedConfig.Clusters).To(HaveKeyWithValue(target, expectedCluster))
			}
			Expect(len(generatedConfig.Clusters)).To(Equal(len(ospreys)))
		})

		It("contains a user per osprey", func() {
			for _, osprey := range ospreys {
				authInfoID := osprey.OspreyconfigTargetName()
				expectedAuthInfo := osprey.ToKubeconfigUserWithoutToken()
				expectedAuthInfo.LocationOfOrigin = ospreyconfig.Kubeconfig
				Expect(generatedConfig.AuthInfos).To(HaveKey(authInfoID))
				Expect(generatedConfig.AuthInfos[authInfoID]).To(WithTransform(WithoutToken, Equal(expectedAuthInfo)))
				Expect(osprey.ToGroupClaims(generatedConfig.AuthInfos[authInfoID])).To(BeEquivalentTo([]string{"admins", "developers"}), "Is a valid token")
			}
			Expect(len(generatedConfig.AuthInfos)).To(Equal(len(ospreys)))
		})

		It("contains a context per osprey", func() {
			for _, osprey := range ospreys {
				kcontext := osprey.ToKubeconfigContext()
				kcontext.LocationOfOrigin = ospreyconfig.Kubeconfig
				target := osprey.OspreyconfigTargetName()
				Expect(generatedConfig.Contexts).To(HaveKeyWithValue(target, kcontext))
			}
			// Each context has an alias
			Expect(len(generatedConfig.Contexts)).To(Equal(len(ospreys) * 2))
		})

		It("contains an alias per context", func() {
			for _, osprey := range ospreys {
				kcontext := osprey.ToKubeconfigContext()
				kcontext.LocationOfOrigin = ospreyconfig.Kubeconfig
				targetAlias := osprey.OspreyconfigAliasName()
				Expect(generatedConfig.Contexts).To(HaveKeyWithValue(targetAlias, kcontext))
			}
			// Each alias has a corresponding context
			Expect(len(generatedConfig.Contexts)).To(Equal(len(ospreys) * 2))
		})
	})
})
