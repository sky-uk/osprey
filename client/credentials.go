package client

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/sky-uk/osprey/common"

	"golang.org/x/crypto/ssh/terminal"
)

// LoginCredentials represents user credentials
type LoginCredentials struct {
	// Username username of user intending to login
	Username string
	// Password the password for user
	Password string
}

// GetCredentials loads the credentials from the terminal or stdin.
func GetCredentials() (*LoginCredentials, error) {
	if terminal.IsTerminal(int(syscall.Stdin)) {
		return consumeCredentials(hiddenInput)
	}
	return consumeCredentials(common.Input)
}

func consumeCredentials(pwdInputFunc func(string, *bufio.Reader) (string, error)) (credentials *LoginCredentials, err error) {
	var username, password string
	reader := bufio.NewReader(os.Stdin)
	if username, err = common.Read("username", "Username: ", reader, common.Input); err != nil {
		return nil, err
	}
	if password, err = common.Read("password", "Password: ", reader, pwdInputFunc); err != nil {
		return nil, err
	}
	return &LoginCredentials{Username: username, Password: password}, nil
}

func hiddenInput(inputName string, reader *bufio.Reader) (string, error) {
	passwordBytes, err := terminal.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err == nil {
		return strings.TrimSpace(string(passwordBytes)), nil
	}
	return "", fmt.Errorf("failed to read %s: %v", inputName, err)
}
