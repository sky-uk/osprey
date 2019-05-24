package clitest

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/onsi/gomega"
)

// NewAsyncCommand returns an instance of an AsyncTestCommand
func NewAsyncCommand(name string, args ...string) AsyncTestCommand {
	return &asyncCommandWrapper{Name: name, Args: args}
}

// AsyncTestCommand defines the behaviour of a long running command.
type AsyncTestCommand interface {
	// Run starts the wrapped command on a new goroutine and waits for it to finish.
	// The goroutine will finish if the process comes to a stop, or the Stop() method
	// is called
	Run()
	// AssertStillRunning checks for the command to still be running.
	AssertStillRunning()
	// Stop signals the command to stop and waits for it to finish.
	Stop()
	// AssertStoppedRunning checks for the command to have finished.
	AssertStoppedRunning()
	// AssertSuccess checks for the command to have finished and asserts success.
	AssertSuccess()
	// AssertFailure checks for the command to have finished and asserts failure.
	AssertFailure()
	// Successful returns true if the command dos not have an error.
	Successful() bool
	// Failed returns true if the command dos not have an error.
	Failed() bool
	// GetOutput returns the stdout and stderr of the command.
	GetOutput() string
	// PrintOutput prints the output of the command to standard output.
	PrintOutput()
	// Error returns the error for the command if one exists.
	Error() error
}

// asyncCommandWrapper wraps an OS process to be ran asynchronously
type asyncCommandWrapper struct {
	Name         string
	Args         []string
	cmd          *exec.Cmd
	finished     bool  // guarded by mutex
	error        error // guarded by mutex
	output       io.Reader
	stopFlag     sync.WaitGroup
	finishedFlag sync.WaitGroup
	sync.Mutex
}

func (c *asyncCommandWrapper) Run() {
	c.cmd = exec.Command(c.Name, c.Args...)
	buf := safeBuffer{}
	c.output = &buf
	c.cmd.Stdout = &buf
	c.cmd.Stderr = &buf
	buf.Write([]byte("*** SERVER STARTED\n"))
	buf.Write([]byte(fmt.Sprintf("%s", c.cmd.Args)))
	err := c.cmd.Start()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	c.stopFlag.Add(1)
	c.finishedFlag.Add(1)

	// signalling goroutine
	go func() {
		c.stopFlag.Wait()
		c.cmd.Process.Signal(syscall.SIGTERM)
		c.finishedFlag.Wait()
		buf.Write([]byte("*** SERVER STOPPED\n"))
	}()

	// process watcher
	go func() {
		err := c.cmd.Wait()
		c.Lock()
		defer c.Unlock()
		c.finished = true
		c.error = err
		c.finishedFlag.Done()
	}()

	// give it time to start
	time.Sleep(50 * time.Millisecond)
}

func (c *asyncCommandWrapper) AssertStillRunning() {
	c.Lock()
	defer c.Unlock()
	assertNoExitError(c.GetOutput(), c.error)
	gomega.Expect(c.finished).To(gomega.BeFalse(), "Server should be running")
}

func (c *asyncCommandWrapper) Stop() {
	c.stopFlag.Done()
	// wait for it to finish
	c.finishedFlag.Wait()
}

func (c *asyncCommandWrapper) AssertStoppedRunning() {
	c.Lock()
	defer c.Unlock()
	assertNoExitError(c.GetOutput(), c.error)
	gomega.Expect(c.finished).To(gomega.BeTrue(), "Server should have stopped")
}

func (c *asyncCommandWrapper) AssertSuccess() {
	c.Lock()
	defer c.Unlock()
	assertNoExitError(c.GetOutput(), c.error)
	gomega.Expect(c.finished).To(gomega.BeTrue(), "should have finished running")
}

func (c *asyncCommandWrapper) AssertFailure() {
	c.Lock()
	defer c.Unlock()
	assertExitError(c.error)
	gomega.Expect(c.finished).To(gomega.BeTrue(), "should have finished running")
}

func (c *asyncCommandWrapper) Successful() bool {
	return c.error == nil
}

func (c *asyncCommandWrapper) Failed() bool {
	return !c.Successful()
}

func (c *asyncCommandWrapper) GetOutput() string {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(c.output)
	gomega.Expect(err).To(gomega.BeNil())
	return fmt.Sprintf("%s\n", string(buf.Bytes()))
}

func (c *asyncCommandWrapper) PrintOutput() {
	fmt.Println("--- Output ---")
	fmt.Printf(c.GetOutput())
	fmt.Println("--- End Output ---")
}

func (c *asyncCommandWrapper) Error() error {
	c.Lock()
	defer c.Unlock()
	return c.error
}

type safeBuffer struct {
	buf bytes.Buffer
	sync.Mutex
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.Lock()
	defer s.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) Read(p []byte) (n int, err error) {
	s.Lock()
	defer s.Unlock()
	return s.buf.Read(p)
}
