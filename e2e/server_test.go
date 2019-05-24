package e2e

import (
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
		localDex *dextest.TestDex
	)

	BeforeEach(func() {
		resetDefaults()
		localDex, err = dextest.Start(testDir, dexPort, "workstation", ldapServer)
		Expect(err).ToNot(HaveOccurred(), "workstation dex should start")
	})

	AfterEach(func() {
		dextest.Stop(localDex)
	})

	It("starts and stops an osprey without TLS", func() {
		osprey := ospreytest.Start(testDir, false, ospreyPort, localDex)
		osprey.AssertStillRunning()

		time.Sleep(100 * time.Millisecond)

		osprey.Stop()
		osprey.AssertStoppedRunning()

		osprey.AssertSuccess()
	})

	It("starts and stops an osprey with TLS", func() {
		osprey := ospreytest.Start(testDir, true, ospreyPort, localDex)
		osprey.AssertStillRunning()

		time.Sleep(100 * time.Millisecond)

		osprey.Stop()
		osprey.AssertStoppedRunning()

		osprey.AssertSuccess()
	})

})
