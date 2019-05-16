package oidc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"os"
	"strings"
)

func consumeToken() (token string, err error) {
	reader := bufio.NewReader(os.Stdin)
	if token, err = read("username", "Token: ", reader, input); err != nil {
		return "", err
	}
	return token, nil
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

func parseError(err error) string {
	e, ok := err.(*oauth2.RetrieveError)
	if ok {
		eResp := make(map[string]string)
		_ = json.Unmarshal(e.Body, &eResp)
		return eResp["error"]
	}
	return ""
}
