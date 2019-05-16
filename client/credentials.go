package client

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

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
	return consumeCredentials(input)
}

func consumeCredentials(pwdInputFunc func(string, *bufio.Reader) (string, error)) (credentials *LoginCredentials, err error) {
	var username, password string
	reader := bufio.NewReader(os.Stdin)
	if username, err = read("username", "Username: ", reader, input); err != nil {
		return nil, err
	}
	if password, err = read("password", "Password: ", reader, pwdInputFunc); err != nil {
		return nil, err
	}
	return &LoginCredentials{Username: username, Password: password}, nil
}

func read(name, prompt string, reader *bufio.Reader, inputFunc func(string, *bufio.Reader) (string, error)) (string, error) {
	fmt.Print(prompt)
	return inputFunc(name, reader)
}

func input(inputName string, reader *bufio.Reader) (string, error) {
	var err error
	if value, err := reader.ReadString('\n'); err == nil {
		value = strings.TrimSpace(value)
		return value, nil
	}
	return "", fmt.Errorf("failed to read %s: %v", inputName, err)
}

func hiddenInput(inputName string, reader *bufio.Reader) (string, error) {
	passwordBytes, err := terminal.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err == nil {
		return strings.TrimSpace(string(passwordBytes)), nil
	}
	return "", fmt.Errorf("failed to read %s: %v", inputName, err)
}
