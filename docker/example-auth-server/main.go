package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
)

var _ auth.AuthorizationServer = (*ExampleAuthHandler)(nil)

type ExampleAuthHandler struct{}

var (
	CheckUnauthorized = &auth.CheckResponse{
		Status: &status.Status{
			Code: int32(code.Code_UNAUTHENTICATED),
		},
		HttpResponse: &auth.CheckResponse_DeniedResponse{
			DeniedResponse: &auth.DeniedHttpResponse{
				Status: &typev3.HttpStatus{
					Code: typev3.StatusCode_Unauthorized,
				},
				Body: "Unauthorized",
			},
		},
	}
	CheckBasicUnauthorized = &auth.CheckResponse{
		Status: &status.Status{
			Code: int32(code.Code_UNAUTHENTICATED),
		},
		HttpResponse: &auth.CheckResponse_DeniedResponse{
			DeniedResponse: &auth.DeniedHttpResponse{
				Status: &typev3.HttpStatus{
					Code: typev3.StatusCode_Unauthorized,
				},
				Headers: []*core.HeaderValueOption{
					{Header: &core.HeaderValue{Key: "WWW-Authenticate", Value: `Basic realm="Access to upstream", charset="UTF-8"`}},
				},
				Body: "Unauthorized",
			},
		},
	}
)

func newPlusHeaders(headers map[string]string) []*core.HeaderValueOption {
	var result []*core.HeaderValueOption
	for name, value := range headers {
		result = append(result, &core.HeaderValueOption{
			Header: &core.HeaderValue{
				Key:   "+" + name,
				Value: value,
			},
		})
	}

	return result
}

func (h *ExampleAuthHandler) Check(ctx context.Context, req *auth.CheckRequest) (*auth.CheckResponse, error) {
	// see https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/auth/v3/external_auth.proto
	authSchema := req.Attributes.ContextExtensions["auth-scheme"]
	if authSchema == "basic" {
		if headerAuth, exists := req.Attributes.Request.Http.Headers["authorization"]; !exists {
			return CheckBasicUnauthorized, nil
		} else if headerAuth != "Basic Z3Vlc3Q6Z3Vlc3Q=" {
			return CheckBasicUnauthorized, nil
		}
	} else {
		return CheckUnauthorized, nil
	}

	return &auth.CheckResponse{
		Status: &status.Status{
			Code: int32(code.Code_OK),
		},
		HttpResponse: &auth.CheckResponse_OkResponse{
			OkResponse: &auth.OkHttpResponse{
				Headers: newPlusHeaders(req.Attributes.Request.Http.Headers),
			},
		},
	}, nil
}

func main() {
	var (
		port uint
	)

	// The port that this auth server listens on
	flag.UintVar(&port, "port", 80, "auth server port")
	flag.Parse()

	grpcServer := grpc.NewServer()
	auth.RegisterAuthorizationServer(grpcServer, &ExampleAuthHandler{})

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("auth server listening on %d\n", port)
	if err = grpcServer.Serve(lis); err != nil {
		log.Println(err)
	}
}
