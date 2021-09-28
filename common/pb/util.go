package pb

import (
	"fmt"

	"io/ioutil"
	"net/http"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/status"

	"github.com/jaytaylor/html2text"
	spb "google.golang.org/genproto/googleapis/rpc/status"
)

// ConsumeLoginResponse takes the https response and produces a LoginResponse
// if the response is successful and can be converted, or an error.
func ConsumeLoginResponse(response *http.Response) (*LoginResponse, error) {
	defer response.Body.Close()
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if response.StatusCode == http.StatusOK {
		accessToken := &LoginResponse{}
		err = proto.Unmarshal(data, accessToken)
		if err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		return accessToken, nil
	}
	return nil, HandleErrorResponse(data, response)
}

// ConsumeClusterInfoResponse takes the HTTP response and produces a ClusterInfoResponse
// if the response is successful and can be converted, or an error.
func ConsumeClusterInfoResponse(response *http.Response) (*ClusterInfoResponse, error) {
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		clusterInfo := &ClusterInfoResponse{}
		err = proto.Unmarshal(data, clusterInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		return clusterInfo, nil
	}
	return nil, HandleErrorResponse(data, response)
}

// HandleErrorResponse returns a response that is known to be an error and converts
// it to an error.
func HandleErrorResponse(body []byte, response *http.Response) (err error) {
	if response.Header.Get("Content-Type") == "application/octet-stream" {
		s := &spb.Status{}
		err = proto.Unmarshal(body, s)
		state := status.FromProto(s)
		if err != nil {
			return fmt.Errorf("failed to parse pb error response: %w", err)
		}
		return state.Err()
	}
	responseText, err := html2text.FromString(string(body), html2text.Options{PrettyTables: true})
	if err != nil {
		return fmt.Errorf("failed to parse html error response: %w", err)
	}
	return fmt.Errorf("\n%s", responseText)
}
