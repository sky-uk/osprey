package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/osprey/e2e/ospreytest"

	"os"

	"github.com/sky-uk/osprey/client/kubeconfig"
	"github.com/sky-uk/osprey/e2e/clitest"
	clientgo "k8s.io/client-go/tools/clientcmd/api"
)

var _ = Describe("Login", func() {
	var login *clitest.CommandWrapper

	BeforeEach(func() {
		defaultGroup = ""
		targetGroup = ""
		targetGroupFlag = ""
	})

	JustBeforeEach(func() {
		setupOspreyClientFlags()

		login = Client("user", "login", ospreyconfigFlag, targetGroupFlag)
	})

	AfterEach(func() {
		cleanup()
	})

	It("fails to login with invalid credentials", func() {
		login.LoginAndAssertFailure("admin", "wrong")
	})

	It("logs in successfully with good credentials", func() {
		login.LoginAndAssertSuccess("jane", "foo")
	})

	It("creates a kubeconfig file on the specified location", func() {
		login.LoginAndAssertSuccess("jane", "foo")

		Expect(ospreyconfig.Kubeconfig).To(BeAnExistingFile())
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
			generatedConfig      *clientgo.Config
			expectedEnvironments []string
		)

		AssertKubeconfigContents := func() {
			JustBeforeEach(func() {
				login.LoginAndAssertSuccess("jane", "foo")

				err := kubeconfig.LoadConfig(ospreyconfig.Kubeconfig)
				Expect(err).To(BeNil(), "successfully creates a kubeconfig")
				generatedConfig, err = kubeconfig.GetConfig()
				Expect(err).To(BeNil(), "successfully creates a kubeconfig")
			})

			It("logs in to the expected targets", func() {
				for _, expectedEnv := range expectedEnvironments {
					Expect(generatedConfig.Clusters).To(HaveKey(OspreyconfigTargetName(expectedEnv)))
				}
			})

			It("contains a cluster per osprey", func() {
				for _, osprey := range targetedOspreys {
					expectedCluster := osprey.ToKubeconfigCluster()
					expectedCluster.LocationOfOrigin = ospreyconfig.Kubeconfig
					target := osprey.OspreyconfigTargetName()
					Expect(generatedConfig.Clusters).To(HaveKeyWithValue(target, expectedCluster))
				}
				Expect(len(generatedConfig.Clusters)).To(Equal(len(targetedOspreys)), "expected number of clusters")
			})

			It("contains a user per osprey", func() {
				for _, osprey := range targetedOspreys {
					authInfoID := osprey.OspreyconfigTargetName()
					expectedAuthInfo := osprey.ToKubeconfigUserWithoutToken()
					expectedAuthInfo.LocationOfOrigin = ospreyconfig.Kubeconfig
					Expect(generatedConfig.AuthInfos).To(HaveKey(authInfoID))
					Expect(generatedConfig.AuthInfos[authInfoID]).To(WithTransform(WithoutToken, Equal(expectedAuthInfo)))
					Expect(osprey.ToGroupClaims(generatedConfig.AuthInfos[authInfoID])).To(BeEquivalentTo([]string{"admins", "developers"}), "Is a valid token")
				}
				Expect(len(generatedConfig.AuthInfos)).To(Equal(len(targetedOspreys)), "expected number of users")
			})

			It("contains a context per osprey", func() {
				for _, osprey := range targetedOspreys {
					kcontext := osprey.ToKubeconfigContext()
					kcontext.LocationOfOrigin = ospreyconfig.Kubeconfig
					target := osprey.OspreyconfigTargetName()
					Expect(generatedConfig.Contexts).To(HaveKeyWithValue(target, kcontext))
				}
				// Each context has an alias
				Expect(len(generatedConfig.Contexts)).To(Equal(len(targetedOspreys)*2), "expected number of contexts")
			})

			It("contains an alias per context", func() {
				for _, osprey := range targetedOspreys {
					kcontext := osprey.ToKubeconfigContext()
					kcontext.LocationOfOrigin = ospreyconfig.Kubeconfig
					targetAlias := osprey.OspreyconfigAliasName()
					Expect(generatedConfig.Contexts).To(HaveKeyWithValue(targetAlias, kcontext))
				}
				// Each alias has a corresponding context
				Expect(len(generatedConfig.Contexts)).To(Equal(len(targetedOspreys)*2), "expected number of alias")
			})
		}

		Context("no group provided", func() {
			Context("no default group", func() {
				BeforeEach(func() {
					defaultGroup = ""
					expectedEnvironments = []string{"local"}
				})

				AssertKubeconfigContents()
			})

			Context("with default group", func() {
				BeforeEach(func() {
					defaultGroup = "production"
					expectedEnvironments = []string{"prod"}
				})

				AssertKubeconfigContents()
			})
		})

		Context("group provided", func() {
			BeforeEach(func() {
				targetGroup = "development"
				expectedEnvironments = []string{"dev", "stage"}
			})

			AssertKubeconfigContents()
		})

		Context("non existent group provided", func() {
			BeforeEach(func() {
				targetGroup = "non_existent"
			})

			It("displays error", func() {
				login.LoginAndAssertFailure("jane", "foo")

				_, err := os.Stat(ospreyconfig.Kubeconfig)
				Expect(os.IsNotExist(err)).To(BeTrue())

				Expect(login.GetOutput()).To(ContainSubstring("Group not found"))
			})
		})
	})

})
