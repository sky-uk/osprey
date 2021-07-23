package common

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strings"
)

// Read is a helper function to read input from stdin
func Read(name, prompt string, reader *bufio.Reader, inputFunc func(string, *bufio.Reader) (string, error)) (string, error) {
	fmt.Print(prompt)
	return inputFunc(name, reader)
}

// Input is the buffered reader for reading from stdin
func Input(inputName string, reader *bufio.Reader) (string, error) {
	var err error
	if value, err := reader.ReadString('\n'); err == nil {
		value = strings.TrimSpace(value)
		return value, nil
	}
	return "", fmt.Errorf("failed to read %s: %v", inputName, err)
}

// ReadAndEncodeFile load the file contents and base64 encodes it
func ReadAndEncodeFile(file string) (string, error) {
	contents, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(contents), nil
}
