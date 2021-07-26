package cmd

import (
	"context"
	"encoding/base64"
	"io/ioutil"
	"os"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func createTempFile(contents string) string {
	fileContents := []byte("foo")
	f, err := ioutil.TempFile("", "server-ca-data-")
	Expect(err).ToNot(HaveOccurred())
	f.Write([]byte(fileContents))
	f.Close()
	return f.Name()
}

func dummyClientset(_ string) (kubernetes.Interface, error) {
	Expect(false).To(Equal(true))
	return nil, nil
}

var _ = Describe("Serve command", func() {
	Describe("helper functions", func() {
		Describe("computeApiServerCa", func() {
			BeforeEach(func() {
				apiServerURL = "http://example.com"
			})
			It("should use a file if told to do so", func() {
				apiServerCASource = "file"
				apiServerCA = createTempFile("foo")
				defer os.Remove(apiServerCA)
				Expect(computeAPIServerCA(dummyClientset)).To(Succeed())
				Expect(apiServerCAData).To(Equal(base64.StdEncoding.EncodeToString([]byte("foo"))))
			})
			It("should use the configmap if told to do so", func() {
				apiServerCASource = "config-map"
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
				Expect(computeAPIServerCA(func(_ string) (kubernetes.Interface, error) { return cs, nil })).To(Succeed())
				Expect(apiServerCAData).To(Equal("foobar"))
			})

			It("should return an error if an unknown source is specified", func() {
				apiServerCASource = "foo"
				Expect(computeAPIServerCA(dummyClientset)).To(MatchError("apiServerCASource argument must be file, config-map, or in-cluster, but it was: foo"))
			})
		})
	})
})
