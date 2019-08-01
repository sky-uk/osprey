package e2e

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/osprey/e2e/dextest"
	"github.com/sky-uk/osprey/e2e/ospreytest"
)

var _ = Describe("Server", func() {
	const (
		dexPort    = int32(13980)
		ospreyPort = int32(13981)
	)

	var (
		localDex    *dextest.TestDex
		localOsprey *ospreytest.TestOsprey
	)

	BeforeEach(func() {
		resetDefaults()
		localDex, err = dextest.Start(testDir, dexPort, "workstation", ldapServer)
		Expect(err).ToNot(HaveOccurred(), "workstation dex should start")
	})

	AfterEach(func() {
		dextest.Stop(localDex)
	})

	startLocalOsprey := func(useTLS bool) {
		localOsprey = ospreytest.Start(testDir, useTLS, ospreyPort, localDex)
		time.Sleep(100 * time.Millisecond)
		localOsprey.AssertStillRunning()
	}

	Context("Start and stop osprey", func() {
		AfterEach(func() {
			localOsprey.Stop()
			localOsprey.AssertStoppedRunning()
			localOsprey.AssertSuccess()
		})

		Specify("With TLS", func() {
			startLocalOsprey(true)
		})

		Specify("Without TLS", func() {
			startLocalOsprey(false)
		})
	})

	Context("Health check", func() {
		AfterEach(func() {
			localOsprey.Stop()
			localOsprey.AssertStoppedRunning()
			localOsprey.AssertSuccess()
		})

		It("Should return ok when healthy", func() {
			startLocalOsprey(true)

			resp, err := localOsprey.CallHealthcheck()

			Expect(err).To(BeNil(), "called healthcheck")
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("Should return unavailable when dex not started", func() {
			dextest.Stop(localDex)

			startLocalOsprey(true)

			resp, err := localOsprey.CallHealthcheck()

			Expect(err).To(BeNil(), "called healthcheck")
			Expect(resp.StatusCode).To(Equal(http.StatusServiceUnavailable))
		})

		It("Should return available when dex is up", func() {
			By("Dex not started")
			dextest.Stop(localDex)
			startLocalOsprey(true)

			resp, err := localOsprey.CallHealthcheck()
			Expect(err).To(BeNil(), "called healthcheck")
			Expect(resp.StatusCode).To(Equal(http.StatusServiceUnavailable))

			By("Dex comes up")
			localDex, err = dextest.Restart(localDex)
			time.Sleep(100 * time.Millisecond)
			Expect(err).ToNot(HaveOccurred(), "dex restarted")
			localDex.RegisterClient(localOsprey.Environment, localOsprey.Secret, fmt.Sprintf("%s/callback", localOsprey.URL), localDex.Environment)

			resp, err = localOsprey.CallHealthcheck()

			localOsprey.PrintOutput()
			Expect(err).To(BeNil(), "called healthcheck")
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

	})
})
