package clitest

import (
	"fmt"
	"os/exec"
	"strings"
)

// NewLoginCommand creates a LoginCommand
func NewLoginCommand(name string, args ...string) LoginCommand {
	return &loginCommandWrapper{
		commandWrapper: &commandWrapper{Name: name, Args: append([]string{"--debug"}, args...)},
	}
}

// LoginCommand adds login behaviour to a TestCommand.
type LoginCommand interface {
	TestCommand
	LoginAndAssertSuccess(username, password string)
	LoginAndAssertFailure(username, password string)
	WithCredentials(username, password string) LoginCommand
}

// loginCommandWrapper wraps an OS process that set pass User and Password to stdin before execution.
type loginCommandWrapper struct {
	*commandWrapper
	User     string
	Password string
}

// Run executes the command without checking its result.
// It passes User and Password to the command's stdin.
func (c *loginCommandWrapper) Run() {
	c.cmd = exec.Command(c.Name, c.Args...)
	c.cmd.Stdin = strings.NewReader(fmt.Sprintf("%s\n%s\n", c.User, c.Password))
	c.run()
}

func (c *loginCommandWrapper) LoginAndAssertSuccess(username, password string) {
	c.User = username
	c.Password = password
	c.RunAndAssertSuccess()
}

func (c *loginCommandWrapper) RunAndAssertSuccess() {
	c.Run()
	assertNoExitError(c.stderr, c.error)
}

func (c *loginCommandWrapper) LoginAndAssertFailure(username, password string) {
	c.User = username
	c.Password = password
	c.RunAndAssertFailure()
}

func (c *loginCommandWrapper) RunAndAssertFailure() {
	c.Run()
	assertExitError(c.error)
}

func (c *loginCommandWrapper) WithCredentials(username, password string) LoginCommand {
	c.User = username
	c.Password = password
	return c
}
