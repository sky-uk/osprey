package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/osprey/server/osprey"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var signals chan os.Signal

// NewServer creates a new Server definition with an empty ServeMux
func NewServer(port int32, tlsCertFile, tlsKeyFile string, shutdownGracePeriod time.Duration) *Server {
	return &Server{
		addr:                fmt.Sprintf("0.0.0.0:%d", port),
		shutdownGracePeriod: shutdownGracePeriod,
		tlsCertFile:         tlsCertFile,
		tlsCertKey:          tlsKeyFile,
		mux:                 http.NewServeMux(),
	}
}

// Start starts a new HTTP server listening at the specified port. If the server configuration
// contains tls data, it will start the server with TLS enabled.
// Start is a blocking method that listens for SIGINT or SIGTERM to start a graceful shutdown,
// with a timeout specified in the server configuration.
func (s *Server) Start() error {
	httpServer := setup(s)
	go func() {
		var err error
		if s.tlsCertFile != "" {
			log.Infof("Starting to listen at: https://%s", s.addr)
			err = httpServer.ListenAndServeTLS(s.tlsCertFile, s.tlsCertKey)
		} else {
			log.Infof("Starting to listen at: http://%s", s.addr)
			err = httpServer.ListenAndServe()
		}
		if err != nil {
			log.Fatalf("Failed to start https server: %v", err)
		}
	}()
	return gracefulShutdown(httpServer, s.shutdownGracePeriod)
}

func gracefulShutdown(s *http.Server, timeout time.Duration) error {
	signals = make(chan os.Signal, 1)
	// SIGTERM is used by Kubernetes to gracefully stop pods.
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	<-signals

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	log.Infof("Shutdown starting with a grace period of %s", timeout)
	return s.Shutdown(ctx)
}

// Server contains the configuration for the HTTP server
type Server struct {
	addr                string
	shutdownGracePeriod time.Duration
	tlsCertFile         string
	tlsCertKey          string
	mux                 *http.ServeMux
}

// RegisterService binds the http endpoints to the Osprey services
// "/access-token" -> Osprey.RetrieveClusterDetailsAndAuthTokens()
func (s *Server) RegisterService(service osprey.Osprey) {
	s.mux.Handle("/access-token", handleAccessToken(service))
	s.mux.Handle("/callback", handleCallback(service))
	s.mux.Handle("/healthz", handleHealthcheck())
}

func setup(server *Server) *http.Server {
	return &http.Server{
		Addr:    server.addr,
		Handler: server.mux,
	}
}

func handleAccessToken(osprey osprey.Osprey) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/octet-stream")
		username, password, _ := r.BasicAuth()
		response, err := osprey.GetAccessToken(context.Background(), username, password)
		handleResponse(w, response, err)
	}
}

func handleCallback(osprey osprey.Osprey) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var response proto.Message
		var err error
		switch r.Method {
		case http.MethodGet:
			var code, state, errMsg string
			errMsg = r.FormValue("error")
			if errMsg != "" {
				errMsg = fmt.Sprintf("%s: %s", errMsg, r.FormValue("error_description"))
			} else {
				code = r.FormValue("code")
				state = r.FormValue("state")
			}
			response, err = osprey.Authorise(r.Context(), code, state, errMsg)

		default:
			err = status.Error(codes.InvalidArgument, "Method not implemented")
		}
		handleResponse(w, response, err)
	}
}

func handleHealthcheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Health check passed!")
	}
}

func handleResponse(w http.ResponseWriter, response proto.Message, err error) {
	if err == nil {
		if response == nil {
			w.WriteHeader(http.StatusOK)
		}
		data, err := proto.Marshal(response)
		if err == nil {
			w.Write(data)
			return
		}
		errMsg := fmt.Sprintf("Failed to marshal success response: %v", err)
		log.Error(errMsg)
		err = status.Error(codes.Internal, errMsg)
	}
	writeError(err, w)
}

func writeError(err error, w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	pbErr, ok := status.FromError(err)
	if !ok {
		errMsg := fmt.Sprintf("Unexpected error: %v", err)
		log.Error(errMsg)
		pbErr = status.New(codes.Unknown, errMsg)
	}
	data, err := proto.Marshal(pbErr.Proto())
	if err == nil {
		w.Write(data)
		return
	}
	errMsg := fmt.Sprintf("Failed to marshal error response: %v", err)
	log.Error(errMsg)
	http.Error(w, errMsg, http.StatusInternalServerError)
}
