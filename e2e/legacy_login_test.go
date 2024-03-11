package e2e

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/osprey/v2/client/kubeconfig"
	"github.com/sky-uk/osprey/v2/e2e/clitest"
	. "github.com/sky-uk/osprey/v2/e2e/ospreytest"
	clientgo "k8s.io/client-go/tools/clientcmd/api"
)

var _ = Describe("Sanity check Login using a legacy osprey config file format", func() {
	var login clitest.LoginCommand

	BeforeEach(func() {
		resetDefaults()
	})

	JustBeforeEach(func() {
		setupClientForEnvironments(ospreyProviderName, environmentsToUse, "", "", false)
		login = Login("user", "login", legacyOspreyconfigFlag, targetGroupFlag, "--disable-browser-popup")
	})

	AfterEach(func() {
		cleanup()
	})

	It("fails to login with invalid credentials when using a legacy osprey config file", func() {
		login.LoginAndAssertFailure("admin", "wrong")
	})

	It("logs in successfully with good credentials when using a legacy osprey config file", func() {
		login.LoginAndAssertSuccess("jane", "foo")
	})

	It("creates a kubeconfig file on the specified location when using a legacy osprey config file", func() {
		login.LoginAndAssertSuccess("jane", "foo")
		Expect(ospreyconfig.LegacyConfig.Kubeconfig).To(BeAnExistingFile())
	})

	It("logs in with certificate-authority-data when using a legacy config file", func() {
		caDataConfig, err := BuildCADataConfig(testDir, ospreyProviderName, ospreys, true, "", "", "", false)
		Expect(err).To(BeNil(), "Creates the osprey config")
		caDataConfigFlag := "--ospreyconfig=" + caDataConfig.LegacyConfigFile
		caDataLogin := Login("user", "login", caDataConfigFlag)

		caDataLogin.LoginAndAssertSuccess("jane", "foo")
	})

	It("logs in overriding certificate-authority with certificate-authority-data when using a legacy osprey config file", func() {
		caDataConfig, err := BuildCADataConfig(testDir, ospreyProviderName, ospreys, true, dexes[0].DexCA, "", "", false)
		Expect(err).To(BeNil(), "Creates the osprey config")
		caDataConfigFlag := "--ospreyconfig=" + caDataConfig.LegacyConfigFile
		caDataLogin := Login("user", "login", caDataConfigFlag)

		caDataLogin.LoginAndAssertSuccess("jane", "foo")
	})

	It("does not allow fetching CA from API Server for Osprey targets when using a legacy osprey config file", func() {
		caDataConfig, err := BuildCADataConfig(testDir, ospreyProviderName, ospreys, true,
			dexes[0].DexCA, "", fmt.Sprintf("http://localhost:%d", apiServerPort), false)
		Expect(err).To(BeNil(), "Creates the osprey config")
		caDataConfigFlag := "--ospreyconfig=" + caDataConfig.LegacyConfigFile
		caDataLogin := Login("user", "login", caDataConfigFlag)

		caDataLogin.LoginAndAssertFailure("jane", "foo")
		Expect(caDataLogin.GetOutput()).To(ContainSubstring("Osprey targets may not fetch the CA from the API Server"))
	})

	Context("kubeconfig file validations when using a legacy osprey config file", func() {
		var (
			generatedConfig      *clientgo.Config
			expectedEnvironments []string
		)

		AssertKubeconfigContents := func() {
			JustBeforeEach(func() {
				login.LoginAndAssertSuccess("jane", "foo")
				Expect(ospreyconfig.LegacyConfig.Kubeconfig).To(BeAnExistingFile())

				err := kubeconfig.LoadConfig(ospreyconfig.LegacyConfig.Kubeconfig)
				Expect(err).To(BeNil(), "expected to successfully Load a kubeconfig")
				generatedConfig, err = kubeconfig.GetConfig()
				Expect(err).To(BeNil(), "expected to successfully get a kubeconfig")
			})

			It("logs in to the expected targets", func() {
				for _, expectedEnv := range expectedEnvironments {
					Expect(generatedConfig.Clusters).To(HaveKey(OspreyconfigTargetName(expectedEnv)))
				}
			})

			It("contains a cluster per osprey", func() {
				for _, osprey := range targetedOspreys {
					expectedCluster := osprey.ToKubeconfigCluster(ospreyconfig.LegacyConfig.Kubeconfig)
					target := osprey.OspreyconfigTargetName()
					Expect(generatedConfig.Clusters).To(HaveKeyWithValue(target, expectedCluster))
				}
				Expect(len(generatedConfig.Clusters)).To(Equal(len(targetedOspreys)), "expected number of clusters")
			})

			It("contains a user per osprey", func() {
				for _, osprey := range targetedOspreys {
					expectedAuthInfo := osprey.ToKubeconfigUserWithoutToken(ospreyconfig.LegacyConfig.Kubeconfig)
					authInfoID := osprey.OspreyconfigTargetName()
					Expect(generatedConfig.AuthInfos).To(HaveKey(authInfoID))
					Expect(generatedConfig.AuthInfos[authInfoID]).To(WithTransform(WithoutToken, Equal(expectedAuthInfo)))
					Expect(osprey.ToGroupClaims(generatedConfig.AuthInfos[authInfoID])).To(BeEquivalentTo([]string{"admins", "developers"}), "Is a valid token")
				}
				Expect(len(generatedConfig.AuthInfos)).To(Equal(len(targetedOspreys)), "expected number of users")
			})

			It("contains a context per osprey", func() {
				for _, osprey := range targetedOspreys {
					kcontext := osprey.ToKubeconfigContext(ospreyconfig.LegacyConfig.Kubeconfig)
					target := osprey.OspreyconfigTargetName()
					Expect(generatedConfig.Contexts).To(HaveKeyWithValue(target, kcontext))
				}
				// Each context has an alias
				Expect(len(generatedConfig.Contexts)).To(Equal(len(targetedOspreys)*2), "expected number of contexts")
			})

			It("contains an alias per context", func() {
				for _, osprey := range targetedOspreys {
					kcontext := osprey.ToKubeconfigContext(ospreyconfig.LegacyConfig.Kubeconfig)
					targetAlias := osprey.OspreyconfigAliasName()
					Expect(generatedConfig.Contexts).To(HaveKeyWithValue(targetAlias, kcontext))
				}
				// Each alias has a corresponding context
				Expect(len(generatedConfig.Contexts)).To(Equal(len(targetedOspreys)*2), "expected number of alias")
			})

		}

		Context("context with configured namespace when using a legacy osprey config file", func() {
			JustBeforeEach(func() {
				By("Customizing the generated contexts")
				login.LoginAndAssertSuccess("jane", "foo")
				err = AddCustomNamespaceToContexts("-namespace", ospreyconfig.LegacyConfig.Kubeconfig, targetedOspreys)
				Expect(err).ToNot(HaveOccurred(), "successfully updates kubeconfig contexts")

				By("logging in again")
				login.LoginAndAssertSuccess("jane", "foo")

				err := kubeconfig.LoadConfig(ospreyconfig.LegacyConfig.Kubeconfig)
				Expect(err).To(BeNil(), "successfully creates a kubeconfig")
				generatedConfig, err = kubeconfig.GetConfig()
				Expect(err).To(BeNil(), "successfully creates a kubeconfig")
			})

			It("preserves namespace per context", func() {
				for _, osprey := range targetedOspreys {
					kcontext := osprey.ToKubeconfigContext(ospreyconfig.LegacyConfig.Kubeconfig)
					kcontext.Namespace = osprey.CustomTargetNamespace("-namespace")
					target := osprey.OspreyconfigTargetName()
					Expect(generatedConfig.Contexts).To(HaveKeyWithValue(target, kcontext))
				}
				// Each context has an alias
				Expect(len(generatedConfig.Contexts)).To(Equal(len(targetedOspreys)*2), "expected number of contexts")
			})

			It("preserves namespace per alias", func() {
				for _, osprey := range targetedOspreys {
					kcontext := osprey.ToKubeconfigContext(ospreyconfig.LegacyConfig.Kubeconfig)
					kcontext.Namespace = osprey.CustomAliasNamespace("-namespace")
					targetAlias := osprey.OspreyconfigAliasName()
					Expect(generatedConfig.Contexts).To(HaveKeyWithValue(targetAlias, kcontext))
				}
				// Each alias has a corresponding context
				Expect(len(generatedConfig.Contexts)).To(Equal(len(targetedOspreys)*2), "expected number of alias")
			})
		})

		Context("no group provided and using a legacy osprey config file", func() {
			Context("no default group and using a legacy osprey config file", func() {
				BeforeEach(func() {
					defaultGroup = ""
					expectedEnvironments = []string{"local"}
				})

				AssertKubeconfigContents()
			})

			Context("with default group and using a legacy osprey config file", func() {
				BeforeEach(func() {
					environmentsToUse = map[string][]string{
						"prod": {"production"},
						"dev":  {"development"},
					}
					defaultGroup = "production"
					expectedEnvironments = []string{"prod"}
				})

				AssertKubeconfigContents()
			})
		})

		Context("group provided and using a legacy osprey config file", func() {
			BeforeEach(func() {
				targetGroup = "development"
				expectedEnvironments = []string{"dev", "stage"}
			})

			AssertKubeconfigContents()
		})

		Context("non existent group provided and using a legacy osprey config file", func() {
			BeforeEach(func() {
				targetGroup = "non_existent"
			})

			It("displays error when login with group not found and using a legacy osprey config file", func() {
				login.LoginAndAssertFailure("jane", "foo")

				_, err := os.Stat(ospreyconfig.LegacyConfig.Kubeconfig)
				Expect(os.IsNotExist(err)).To(BeTrue())

				Expect(login.GetOutput()).To(ContainSubstring("Group not found"))
			})
		})
	})

	Context("output", func() {
		assertSharedOutputTest(func() clitest.TestCommand {
			cmd := Login("user", "login", ospreyconfigFlag, targetGroupFlag)
			return cmd.WithCredentials("jane", "foo")
		})
	})
})
