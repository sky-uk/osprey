package clitest

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/onsi/gomega"
)

// CommandWrapper wraps an OS process to be ran synchronously.
type CommandWrapper struct {
	Cmd    *exec.Cmd
	Error  error
	output string
	stderr string
}

// Run runs the wrapped command without checking its result.
func (c *CommandWrapper) Run() {
	c.run()
}

// RunAndAssertSuccess runs the wrapped command and asserts success.
// Succeeds and prints the command's output if the command finished successfully.
func (c *CommandWrapper) RunAndAssertSuccess() {
	c.run()
	assertNoExitError(c.stderr, c.Error)
}

// RunAndAssertFailure runs the wrapped command and asserts failure.
// Succeeds if the command finished with a failure.
func (c *CommandWrapper) RunAndAssertFailure() {
	c.run()
	assertExitError(c.Error)
}

// LoginAndAssertSuccess provides the username and password to stdin and runs the command and asserts success.
// Succeeds if the command finished successfully.
func (c *CommandWrapper) LoginAndAssertSuccess(username, password string) {
	c.Cmd.Stdin = strings.NewReader(fmt.Sprintf("%s\n%s\n", username, password))
	c.run()
	assertNoExitError(c.stderr, c.Error)
}

// LoginAndAssertFailure provides the username and password to stdin and runs the command and asserts failure.
// Succeeds if the command finished with a failure.
func (c *CommandWrapper) LoginAndAssertFailure(username, password string) {
	c.Cmd.Stdin = strings.NewReader(fmt.Sprintf("%s\n%s\n", username, password))
	c.run()
	assertExitError(c.Error)
}

func (c *CommandWrapper) run() {
	var stderrB bytes.Buffer
	c.Cmd.Stderr = &stderrB
	rawOutput, err := c.Cmd.Output()
	c.output = string(rawOutput)
	c.Error = err
	c.stderr = stderrB.String()
}

// GetOutput returns the stdout and stderr of the command.
func (c *CommandWrapper) GetOutput() string {
	return fmt.Sprintf("%s\n%s", c.output, c.stderr)
}

// PrintOutput prints the output of the command to standard output.
func (c *CommandWrapper) PrintOutput() {
	fmt.Println("--- Output ---")
	fmt.Printf(c.GetOutput())
	fmt.Println("--- End Output ---")
}

// Successful returns true if the command dos not have an error.
func (c *CommandWrapper) Successful() bool {
	return c.Error == nil
}

func assertNoExitError(out string, err error) {
	if _, ok := err.(*exec.ExitError); ok {
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Unexpected exit error: %v\n%s", err, out)
	} else {
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Unexpected error: %v\n%s", err, out)
	}
}

func assertExitError(err error) {
	gomega.Expect(err).To(gomega.HaveOccurred(), "exit error should have occurred")
	gomega.Expect(err).To(gomega.BeAssignableToTypeOf(&exec.ExitError{}), "exit error should have occurred")
}

// AsyncCommandWrapper wraps an OS process to be ran asynchronously
type AsyncCommandWrapper struct {
	Cmd          *exec.Cmd
	finished     bool  // guarded by mutex
	error        error // guarded by mutex
	output       io.Reader
	stopFlag     sync.WaitGroup
	finishedFlag sync.WaitGroup
	sync.Mutex
}

// Successful returns true if the command dos not have an error.
func (c *AsyncCommandWrapper) Successful() bool {
	return c.error == nil
}

// RunAsync starts the wrapped command on a new goroutine and waits for it to finish.
// The goroutine will finish if the process comes to a stop, or the StopAsync() method
// is called
func (c *AsyncCommandWrapper) RunAsync() {
	buf := safeBuffer{}
	c.output = &buf
	c.Cmd.Stdout = &buf
	c.Cmd.Stderr = &buf
	buf.Write([]byte("*** SERVER STARTED\n"))
	buf.Write([]byte(fmt.Sprintf("%s", c.Cmd.Args)))
	err := c.Cmd.Start()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	c.stopFlag.Add(1)
	c.finishedFlag.Add(1)

	// signalling goroutine
	go func() {
		c.stopFlag.Wait()
		c.Cmd.Process.Signal(syscall.SIGTERM)
		c.finishedFlag.Wait()
		buf.Write([]byte("*** SERVER STOPPED\n"))
	}()

	// process watcher
	go func() {
		err := c.Cmd.Wait()
		c.Lock()
		defer c.Unlock()
		c.finished = true
		c.error = err
		c.finishedFlag.Done()
	}()

	// give it time to start
	time.Sleep(50 * time.Millisecond)
}

// StopAsync signals the command to stop and waits for it to finish.
func (c *AsyncCommandWrapper) StopAsync() {
	c.stopFlag.Done()
	// wait for it to finish
	c.finishedFlag.Wait()
}

// Error returns the error for the command if one exists.
func (c *AsyncCommandWrapper) Error() error {
	c.Lock()
	defer c.Unlock()
	return c.error
}

// AssertStillRunning checks for the command to still be running.
// Succeeds if it is running.
func (c *AsyncCommandWrapper) AssertStillRunning() {
	c.Lock()
	defer c.Unlock()
	assertNoExitError(c.getOutput(), c.error)
	gomega.Expect(c.finished).To(gomega.BeFalse(), "Server should be running")
}

// AssertSuccessfullyRan checks for the command to have finished and asserts success.
// Succeeds if no errors occurred.
func (c *AsyncCommandWrapper) AssertSuccessfullyRan() {
	c.Lock()
	defer c.Unlock()
	assertNoExitError(c.getOutput(), c.error)
	gomega.Expect(c.finished).To(gomega.BeTrue(), "should have finished running")
}

// PrintOutput prints the command's output to standard out.
func (c *AsyncCommandWrapper) PrintOutput() {
	fmt.Println("--- Output ---")
	fmt.Printf(c.getOutput())
	fmt.Println("--- End Output ---")
}

func (c *AsyncCommandWrapper) getOutput() string {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(c.output)
	gomega.Expect(err).To(gomega.BeNil())
	return fmt.Sprintf("%s\n", string(buf.Bytes()))
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
