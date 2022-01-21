package clitest

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/onsi/gomega"
)

// TestCommand defines the behaviour for a synchronous OS wrapper.
type TestCommand interface {
	// Run executes the command without checking its result.
	Run()
	// RunAndAssertSuccess executes the wrapped command and expects a successful execution.
	RunAndAssertSuccess()
	// RunAndAssertFailure executes the wrapped command and expects a failed execution.
	RunAndAssertFailure()
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

// NewCommand returns an instance of a TestCommand
func NewCommand(name string, args ...string) TestCommand {
	return &commandWrapper{Name: name, Args: args}
}

// commandWrapper wraps an OS process to be ran synchronously.
type commandWrapper struct {
	Name   string
	Args   []string
	cmd    *exec.Cmd
	error  error
	output string
	stderr string
}

func (c *commandWrapper) Run() {
	c.cmd = exec.Command(c.Name, c.Args...)
	c.run()
}

func (c *commandWrapper) RunAndAssertSuccess() {
	c.Run()
	assertNoExitError(c.stderr, c.error)
}

func (c *commandWrapper) RunAndAssertFailure() {
	c.Run()
	assertExitError(c.error)
}

func (c *commandWrapper) Successful() bool {
	return c.error == nil
}

func (c *commandWrapper) Failed() bool {
	return !c.Successful()
}

func (c *commandWrapper) GetOutput() string {
	return fmt.Sprintf("%s\n%s", c.output, c.stderr)
}

func (c *commandWrapper) PrintOutput() {
	fmt.Println("--- Output ---")
	fmt.Println(c.cmd)
	fmt.Printf(c.GetOutput())
	fmt.Println("--- End Output ---")
}

func (c *commandWrapper) Error() error {
	return c.error
}

func (c *commandWrapper) run() {
	var stderrB bytes.Buffer
	c.cmd.Stderr = &stderrB
	rawOutput, err := c.cmd.Output()
	c.output = string(rawOutput)
	c.error = err
	c.stderr = stderrB.String()
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
