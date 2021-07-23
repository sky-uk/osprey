package cmd_test

import (
	"io/ioutil"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

func TestCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	logrus.SetOutput(ioutil.Discard)
	RunSpecs(t, "Cmd Suite")
}
