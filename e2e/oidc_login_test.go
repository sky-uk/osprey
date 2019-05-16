package e2e

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sky-uk/osprey/client/kubeconfig"
	"k8s.io/client-go/tools/clientcmd/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/osprey/e2e/clitest"
)

const (
	oidcPort           = int(14980)
	oidcClientID       = "some-client-id"
	oidcRedirectURI    = "http://localhost:65525/auth/callback"
	ospreyState        = "as78*sadf$212"
	ospreyBinary       = "osprey"
	azureApplicationID = "123456-123456-123456-123456"
)

var _ = Describe("Login with a cloud provider", func() {

	var (
		userLoginArgs []string
	)

	BeforeEach(func() {
		resetDefaults()
	})

	JustBeforeEach(func() {
		setupClientForEnvironments("azure", environmentsToUse, oidcClientID)
		userLoginArgs = []string{"user", "login", ospreyconfigFlag}
	})

	AfterEach(func() {
		oidcMockServer.Reset()
	})

	getKubeConfig := func() *api.Config {
		err := kubeconfig.LoadConfig(ospreyconfig.Kubeconfig)
		Expect(err).To(BeNil(), "successfully creates a kubeconfig")
		generatedConfig, err := kubeconfig.GetConfig()
		Expect(err).To(BeNil(), "successfully creates a kubeconfig")
		return generatedConfig
	}

	Context("using OIDC callback (--interactive=true)", func() {

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

		It("provides the same JWT token for multiple targets in group", func() {
			setupClientForEnvironments("azure", map[string][]string{"dev": {"development"}, "stage": {"development"}}, oidcClientID)
			targetGroupArgs := append(userLoginArgs, "--group=development")
			login := loginCommand(ospreyBinary, targetGroupArgs...)

			_, err := doOIDCMockRequest("/authorize", oidcClientID, oidcRedirectURI, ospreyState, []string{"api://some-dummy-scope"})
			Expect(err).NotTo(HaveOccurred())

			login.AssertSuccess()

			Expect(oidcMockServer.RequestCount("/authorize")).To(Equal(1))

			kubeconfig := getKubeConfig()
			Expect(kubeconfig.AuthInfos["kubectl.dev"].Token).To(Equal(kubeconfig.AuthInfos["kubectl.stage"].Token))
		})
	})

	Context("using OIDC device-flow authentication (--interactive=false)", func() {

		It("receives a token and decodes the JWT for user details", func() {
			By("logging in", func() {
				nonInteractiveUserCommand := append(userLoginArgs, "--interactive=false")
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

		FIt("provides the same JWT token for multiple targets in group", func() {
			setupClientForEnvironments("azure", map[string][]string{"dev": {"development"}, "stage": {"development"}}, oidcClientID)
			targetGroupArgs := append(userLoginArgs, "--group=development", "--interactive=false")
			login := loginCommand(ospreyBinary, targetGroupArgs...)

			err = doRequestToMockDeviceFlowEndpoint("good_client_id")
			Expect(err).NotTo(HaveOccurred())

			login.AssertSuccess()

			Expect(oidcMockServer.RequestCount("/token")).To(Equal(1))

			kubeconfig := getKubeConfig()
			Expect(kubeconfig.AuthInfos["kubectl.dev"].Token).To(Equal(kubeconfig.AuthInfos["kubectl.stage"].Token))
		})

		It("Polls the token endpoint at server specified intervals when token status is pending", func() {
			setupClientForEnvironments("azure", environmentsToUse, "pending_client_id")
			nonInteractiveUserCommand := append(userLoginArgs, "--interactive=false")
			login := loginCommand(ospreyBinary, nonInteractiveUserCommand...)

			err = doRequestToMockDeviceFlowEndpoint(oidcClientID)
			time.Sleep(2 * time.Second)
			Expect(err).NotTo(HaveOccurred())

			login.AssertSuccess()
			Expect(oidcMockServer.RequestCount("/token")).To(Equal(3))
		})

		It("Handles client id is not authorised error code", func() {
			setupClientForEnvironments("azure", environmentsToUse, "bad_verification_client_id")
			nonInteractiveUserCommand := append(userLoginArgs, "--interactive=false")
			login := loginCommand(ospreyBinary, nonInteractiveUserCommand...)

			err = doRequestToMockDeviceFlowEndpoint("bad_verification_client_id")
			Expect(err).NotTo(HaveOccurred())

			login.AssertFailure()
		})

		It("Handles device code expired error code", func() {
			setupClientForEnvironments("azure", environmentsToUse, "expired_client_id")
			nonInteractiveUserCommand := append(userLoginArgs, "--interactive=false")
			login := loginCommand(ospreyBinary, nonInteractiveUserCommand...)

			err = doRequestToMockDeviceFlowEndpoint("expired_client_id")
			Expect(err).NotTo(HaveOccurred())

			login.AssertFailure()
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
