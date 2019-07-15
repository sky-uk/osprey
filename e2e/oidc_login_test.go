package e2e

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/osprey/e2e/clitest"
	"github.com/sky-uk/osprey/e2e/ospreytest"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	oidcPort           = int(14980)
	oidcClientID       = "some-client-id"
	oidcRedirectURI    = "http://localhost:65525/auth/callback"
	ospreyState        = "as78*sadf$212"
	ospreyBinary       = "osprey"
	azureApplicationID = "123456-123456-123456-123456"
)

var _ = FDescribe("Login with a cloud provider", func() {

	var (
		userLoginArgs       []string
		targets             clitest.TestCommand
		byGroupsFlag        string
		listGroupsFlag      string
		expectedOutputLines []string
	)

	BeforeEach(func() {
		resetDefaults()
		byGroupsFlag = ""
		listGroupsFlag = ""
		expectedOutputLines = []string{}
	})

	JustBeforeEach(func() {
		setupClientForEnvironments("azure", environmentsToUse, "some-client-id")
		userLoginArgs = []string{"user", "login", ospreyconfigFlag}
		targets = ospreytest.Client("config", "targets", ospreyconfigFlag, targetGroupFlag, byGroupsFlag, listGroupsFlag)
	})

	AfterEach(func() {
		oidcMockServer.Reset()
	})

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

		//It("provides a JWT token for multiple targets in group", func() {
		//	groupTargetLoginArgs := append(userLoginArgs, targetGroupFlag)
		//	login := loginCommand(ospreyBinary, groupTargetLoginArgs...)
		//
		//	_, err := doOIDCMockRequest("/authorize", oidcClientID, oidcRedirectURI, ospreyState, []string{"api://some-dummy-scope"})
		//	Expect(err).NotTo(HaveOccurred())
		//
		//	Expect(login.GetOutput()).To(ContainSubstring("asdasds"))
		//})

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

		It("Polls the token endpoint at server specified intervals when token status is pending", func() {
			By("logging in", func() {
				setupClientForEnvironments("azure", environmentsToUse, "pending_client_id")
				nonInteractiveUserCommand := append(userLoginArgs, "--interactive=false")
				login := loginCommand(ospreyBinary, nonInteractiveUserCommand...)

				err = doRequestToMockDeviceFlowEndpoint("pending_client_id")
				time.Sleep(2 * time.Second)
				// check every 1 sec for the expected result
				// keep a max attempt or rimeout
				// if the result is not the expected one after the max time/attempts, fail, else keep trying every x seconds
				Expect(err).NotTo(HaveOccurred())

				login.AssertSuccess()
				Expect(oidcMockServer.RequestCount("/token")).To(Equal(3))
			})
		})
	})



	Context("successfully into multiple targets using a single identity provider for all targets", func() {
		AssertTargetsSuccessfulOutput := func() {
			It("displays the targets in alphabetical order", func() {
				targets.RunAndAssertSuccess()

				Expect(targets.GetOutput()).To(ContainSubstring(strings.Join(expectedOutputLines, "\n")))
				Expect(targets.GetOutput()).To(Equal("lol"))
			})
		}

		By("logging in", func() {
			environmentsToUse = map[string][]string{
				"prod":    {"production"},
				"dev":     {"development"},
				"stage":   {"development"},
				"sandbox": {"sandbox"},
			}
			defaultGroup = "development"
			expectedOutputLines = []string{"Osprey targets:",
				fmt.Sprintf("* %s", ospreytest.OspreyTargetOutput("dev")),
				fmt.Sprintf("  %s", ospreytest.OspreyTargetOutput("prod")),
				fmt.Sprintf("  %s", ospreytest.OspreyTargetOutput("sandbox")),
				fmt.Sprintf("* %s", ospreytest.OspreyTargetOutput("stage")),
			}
		})
		AssertTargetsSuccessfulOutput()
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
