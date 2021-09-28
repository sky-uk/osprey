package e2e

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sky-uk/osprey/e2e/apiservertest"

	"github.com/sky-uk/osprey/client/kubeconfig"
	"k8s.io/client-go/tools/clientcmd/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/osprey/e2e/clitest"
)

const (
	oidcPort        = int(14980)
	oidcClientID    = "some-client-id"
	oidcRedirectURI = "http://localhost:65525/auth/callback"
	ospreyState     = "as78*sadf$212"
	ospreyBinary    = "osprey"
)

var _ = Describe("Login with a cloud provider", func() {

	var (
		userLoginArgs []string
	)

	BeforeEach(func() {
		resetDefaults()
	})

	JustBeforeEach(func() {
		setupClientForEnvironments(azureProviderName, environmentsToUse, oidcClientID, apiServerURL)
		userLoginArgs = []string{"user", "login", ospreyconfigFlag, "--disable-browser-popup"}
	})

	getKubeConfig := func() *api.Config {
		err := kubeconfig.LoadConfig(ospreyconfig.Kubeconfig)
		Expect(err).To(BeNil(), "successfully creates a kubeconfig")
		generatedConfig, err := kubeconfig.GetConfig()
		Expect(err).To(BeNil(), "successfully creates a kubeconfig")
		return generatedConfig
	}

	Context("using OIDC callback (--use-device-code=false)", func() {
		AfterEach(func() {
			oidcTestServer.Reset()
			apiTestServer.Reset()
		})
		It("receives a token and decodes the JWT for user details", func() {
			By("logging in", func() {
				login := loginCommand(ospreyBinary, userLoginArgs...)

				_, err := doOIDCMockRequest("/authorize", oidcClientID, oidcRedirectURI, ospreyState, []string{"api://some-dummy-scope"})
				Expect(err).NotTo(HaveOccurred())

				login.AssertSuccess()
			})

			By("running the user command", func() {
				userCommand := clitest.NewCommand(ospreyBinary, "user", ospreyconfigFlag)
				userCommand.Run()
				Expect(userCommand.GetOutput()).To(ContainSubstring("john.doe@osprey.org"))
			})
		})

		Describe("fetches the CA from the requested location", func() {
			Describe("with api-server not defined in osprey config", func() {
				It("should fetch from the Osprey server", func() {
					login := loginCommand(ospreyBinary, userLoginArgs...)

					_, err := doOIDCMockRequest("/authorize", oidcClientID, oidcRedirectURI, ospreyState, []string{"api://some-dummy-scope"})
					Expect(err).NotTo(HaveOccurred())

					login.AssertSuccess()
				})
			})

			Describe("with api-server defined in osprey config", func() {
				BeforeEach(func() {
					apiServerURL = fmt.Sprintf("http://localhost:%d", apiServerPort)
				})
				It("should fetch from the API Server", func() {
					login := loginCommand(ospreyBinary, userLoginArgs...)

					_, err := doOIDCMockRequest("/authorize", oidcClientID, oidcRedirectURI, ospreyState, []string{"api://some-dummy-scope"})
					Expect(err).NotTo(HaveOccurred())

					login.AssertSuccess()
					Expect(apiTestServer.RequestCount("/api/v1/namespaces/kube-public/configmaps/kube-root-ca.crt")).To(Equal(1))
					kubeconfig := getKubeConfig()
					Expect(kubeconfig.Clusters["kubectl.local"].CertificateAuthorityData).To(ContainSubstring(apiservertest.CaCertIdentifyingPortion))
				})
			})
		})

		It("provides the same JWT token for multiple targets in group for the same provider", func() {
			setupClientForEnvironments(azureProviderName, map[string][]string{"dev": {"development"}, "stage": {"development"}}, oidcClientID, "")
			targetGroupArgs := append(userLoginArgs, "--group=development")
			login := loginCommand(ospreyBinary, targetGroupArgs...)

			_, err := doOIDCMockRequest("/authorize", oidcClientID, oidcRedirectURI, ospreyState, []string{"api://some-dummy-scope"})
			Expect(err).NotTo(HaveOccurred())

			login.AssertSuccess()

			Expect(oidcTestServer.RequestCount("/authorize")).To(Equal(1))

			kubeconfig := getKubeConfig()
			Expect(kubeconfig.AuthInfos["kubectl.dev"].Token).To(Equal(kubeconfig.AuthInfos["kubectl.stage"].Token))
		})
	})

	Context("using OIDC device-flow authentication (--use-device-code=true)", func() {
		AfterEach(func() {
			oidcTestServer.Reset()
		})
		It("receives a token and decodes the JWT for user details", func() {
			By("logging in", func() {
				nonInteractiveUserCommand := append(userLoginArgs, "--use-device-code")
				login := loginCommand(ospreyBinary, nonInteractiveUserCommand...)

				err = doRequestToMockDeviceFlowEndpoint("good_client_id")
				Expect(err).NotTo(HaveOccurred())

				login.AssertSuccess()
			})

			By("running the user command", func() {
				userCommand := clitest.NewCommand(ospreyBinary, "user", ospreyconfigFlag)
				userCommand.Run()
				Expect(userCommand.GetOutput()).To(ContainSubstring("john.doe@osprey.org"))
			})
		})

		It("provides the same JWT token for multiple targets in group for the same provider", func() {
			setupClientForEnvironments(azureProviderName, map[string][]string{"dev": {"development"}, "stage": {"development"}}, oidcClientID, "")
			targetGroupArgs := append(userLoginArgs, "--group=development", "--use-device-code")
			login := loginCommand(ospreyBinary, targetGroupArgs...)

			err = doRequestToMockDeviceFlowEndpoint("good_client_id")
			Expect(err).NotTo(HaveOccurred())

			login.AssertSuccess()

			Expect(oidcTestServer.RequestCount("/token")).To(Equal(1))

			kubeconfig := getKubeConfig()
			Expect(kubeconfig.AuthInfos["kubectl.dev"].Token).To(Equal(kubeconfig.AuthInfos["kubectl.stage"].Token))
		})

		It("Polls the token endpoint at server specified intervals when token status is pending", func() {
			setupClientForEnvironments(azureProviderName, environmentsToUse, "pending_client_id", "")
			useDeviceCodeArgs := append(userLoginArgs, "--use-device-code")
			login := loginCommand(ospreyBinary, useDeviceCodeArgs...)

			err = doRequestToMockDeviceFlowEndpoint(oidcClientID)
			Expect(err).NotTo(HaveOccurred())

			login.EventuallyAssertSuccess(10*time.Second, 1*time.Second)

			Expect(oidcTestServer.RequestCount("/token")).To(Equal(3))
		})

		It("Handles client id is not authorised error code", func() {
			setupClientForEnvironments(azureProviderName, environmentsToUse, "bad_verification_client_id", "")
			useDeviceCodeArgs := append(userLoginArgs, "--use-device-code")
			login := loginCommand(ospreyBinary, useDeviceCodeArgs...)

			err = doRequestToMockDeviceFlowEndpoint("bad_verification_client_id")
			Expect(err).NotTo(HaveOccurred())

			login.AssertFailure()
		})

		It("Handles device code expired error code", func() {
			setupClientForEnvironments(azureProviderName, environmentsToUse, "expired_client_id", "")
			useDeviceCodeArgs := append(userLoginArgs, "--use-device-code")
			login := loginCommand(ospreyBinary, useDeviceCodeArgs...)

			err = doRequestToMockDeviceFlowEndpoint("expired_client_id")
			Expect(err).NotTo(HaveOccurred())

			login.AssertFailure()
		})
	})

	Context("Specifiying the --login-timeout flag", func() {
		It("logs in successfully if the flow is complete before the timeout", func() {
			setupClientForEnvironments(azureProviderName, environmentsToUse, "login_timeout_exceeded_client_id", "")
			timeoutArgs := append(userLoginArgs, "--login-timeout=20s")
			login := loginCommand(ospreyBinary, timeoutArgs...)

			_, err := doOIDCMockRequest("/authorize", oidcClientID, oidcRedirectURI, ospreyState, []string{"api://some-dummy-scope"})
			Expect(err).NotTo(HaveOccurred())

			login.AssertSuccess()
		})

		It("callback flow times out if not logged in within the stipulated time", func() {
			setupClientForEnvironments(azureProviderName, environmentsToUse, "login_timeout_exceeded_client_id", "")
			timeoutArgs := append(userLoginArgs, "--login-timeout=1s")
			login := loginCommand(ospreyBinary, timeoutArgs...)

			login.EventuallyAssertFailure(5*time.Second, 1*time.Second)
			Expect(login.GetOutput()).To(ContainSubstring("exceeded login deadline"))
		})

		It("device-code flow times out if not logged in within the stipulated time", func() {
			setupClientForEnvironments(azureProviderName, environmentsToUse, "login_timeout_exceeded_client_id", "")
			deviceCodeTimeoutArgs := append(userLoginArgs, "--use-device-code=true", "--login-timeout=1s")
			login := loginCommand(ospreyBinary, deviceCodeTimeoutArgs...)

			login.EventuallyAssertFailure(5*time.Second, 1*time.Second)
			Expect(login.GetOutput()).To(ContainSubstring("exceeded device-code login deadline"))
		})
	})
})

func loginCommand(ospreyBinary string, userLoginArgs ...string) clitest.AsyncTestCommand {
	loginAsyncCommand := clitest.NewAsyncCommand(ospreyBinary, userLoginArgs...)
	loginAsyncCommand.Run()
	return loginAsyncCommand
}

func doRequestToMockDeviceFlowEndpoint(clientID string) error {
	time.Sleep(2 * time.Second)
	_, err := http.Get("http://localhost:" + strconv.Itoa(oidcPort) + "/devicecode?&client_id=" + clientID)
	if err != nil {
		return err
	}
	return nil
}

func doOIDCMockRequest(endpoint, clientID, redirectURI, state string, scopes []string) (*http.Response, error) {
	// include sleeps in order for the client's callback webserver to become available, and also to finish processing
	// the requests it does to fetch cluster information.
	client := http.Client{}
	time.Sleep(time.Second)
	httpParameters := &url.Values{
		"response_code": {"code"},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"scope":         {strings.Join(scopes, " ")},
		"state":         {state},
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d%s?%s", oidcPort, endpoint, httpParameters.Encode()), nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch response: %v", err)
	}

	time.Sleep(time.Second)
	return resp, nil
}
