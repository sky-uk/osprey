package util

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
)

// CreateBinaries executes the project's make install task to create the binaries for the e2e tests.
func CreateBinaries() {
	cmd := exec.Command("make", "install")
	cmd.Dir = ProjectDir()
	if out, err := cmd.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("Unable to create osprey binaries: %s: %v", out, err))
	}
}

// ProjectDir returns the root location of the project based on the GOPATH env variable.
func ProjectDir() string {
	return filepath.Join(os.Getenv("PWD"), "..")
}

// TestDataDir returns the location of the ldap test data directory.
func TestDataDir() (string, error) {
	return filepath.Join(ProjectDir(), "e2e", "ldaptest", "testdata"), nil
}

// RandomString creates a random string of size length, containing values fom [a-zA-Z]
func RandomString(length int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var buf bytes.Buffer
	for i := 0; i < length; i++ {
		c := letters[rand.Intn(len(letters))]
		buf.WriteByte(c)
	}
	return buf.String()
}
