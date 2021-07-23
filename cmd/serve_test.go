package cmd

import (
	"context"
	"encoding/base64"
	"io/ioutil"
	"os"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Serve command", func() {
	Describe("helper functions", func() {
		Describe("setApiServerCaDataFromFile", func() {
			It("should set the value of the variable according to the contents of the file", func() {
				fileContents := []byte("foo")
				f, err := ioutil.TempFile("", "server-ca-data-")
				Expect(err).ToNot(HaveOccurred())
				f.Write([]byte(fileContents))
				f.Close()
				defer os.Remove(f.Name())

				apiServerCa = f.Name()
				Expect(setApiServerCaDataFromFile()).To(Succeed())
				Expect(apiServerCaData).To(Equal(base64.StdEncoding.EncodeToString(fileContents)))
			})
		})
		Describe("setApiServerCaFromApi", func() {
			It("should set the data according to the api response", func() {
				configBytes, err := clientcmd.Write(clientcmdapi.Config{
					APIVersion: "v1",
					Clusters: map[string]*clientcmdapi.Cluster{
						"": {
							CertificateAuthorityData: []byte("foobar"),
						},
					},
				})
				Expect(err).ToNot(HaveOccurred())
				cs := fake.NewSimpleClientset()
				_, err = cs.CoreV1().ConfigMaps("kube-public").Create(context.TODO(), &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-info",
						Namespace: "kube-public",
					},
					Data: map[string]string{
						"kubeconfig": string(configBytes),
					},
				}, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(setApiServerCaDataFromApi(cs)).To(Succeed())
				Expect(apiServerCaData).To(Equal("foobar"))
			})
		})

		Describe("computeApiServerCa", func() {
			// FIXME: tricky because of the need to mock getClientsetForUrl
			// It("should fallback to the file if configmap is not found", func() {
			// 	// transitional behavior, will delete this test eventually
			// 	cs := fake.NewSimpleClientset()
			// 	fileContents := []byte("foo")
			// 	f, err := ioutil.TempFile("", "server-ca-data-*")
			// 	Expect(err).ToNot(HaveOccurred())
			// 	f.Write([]byte(fileContents))
			// 	f.Close()
			// 	defer os.Remove(f.Name())

			// 	apiServerCa = f.Name()
			// 	apiServerUrl = "http://example.com"
			// 	Expect(setApiServerCaDataFromApi(cs)).To(Succeed())
			// 	Expect(apiServerCaData).To(Equal("foo"))
			// })
		})
	})
})
