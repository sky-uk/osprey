package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/osprey/e2e/ospreytest"

	"github.com/sky-uk/osprey/client/kubeconfig"
	"github.com/sky-uk/osprey/e2e/clitest"
)

var _ = Describe("Logout", func() {
	var login, logout *clitest.CommandWrapper

	BeforeEach(func() {
		defaultGroup = ""
		targetGroup = ""
		targetGroupFlag = ""
	})

	JustBeforeEach(func() {
		setupOspreyFlags()

		login = Client("user", "login", ospreyconfigFlag, targetGroupFlag)
		logout = Client("user", "logout", ospreyconfigFlag, targetGroupFlag)
	})

	AfterEach(func() {
		cleanup()
	})

	Context("without login", func() {
		It("is a no-op", func() {
			logout.RunAndAssertSuccess()

			kubeconfig.LoadConfig(ospreyconfig.ConfigFile)
			loggedOutConfig, err := kubeconfig.GetConfig()
			Expect(err).To(BeNil(), "no-op")
			Expect(loggedOutConfig.AuthInfos).To(BeEmpty())
		})
	})

	Context("after login", func() {
		JustBeforeEach(func() {
			login.LoginAndAssertSuccess("jane", "foo")
			err := kubeconfig.LoadConfig(ospreyconfig.Kubeconfig)
			Expect(err).To(BeNil(), "successfully creates a kubeconfig")
			_, err = kubeconfig.GetConfig()
			Expect(err).To(BeNil(), "successfully creates a kubeconfig")
		})

		It("removes the tokens for the managed users from the kubeconfig file", func() {
			logout.RunAndAssertSuccess()

			loggedOutConfig, err := kubeconfig.GetConfig()
			Expect(err).To(BeNil(), "successfully updated kubeconfig")
			for _, osprey := range targetedOspreys {
				authInfoID := osprey.OspreyconfigTargetName()
				expectedAuthInfo := osprey.ToKubeconfigUserWithoutToken()
				expectedAuthInfo.LocationOfOrigin = ospreyconfig.Kubeconfig
				Expect(loggedOutConfig.AuthInfos).To(HaveKey(authInfoID))
				Expect(loggedOutConfig.AuthInfos[authInfoID]).To(Equal(expectedAuthInfo), "does not have a token")
			}
			Expect(len(loggedOutConfig.AuthInfos)).To(Equal(len(targetedOspreys)))
		})
	})
})
