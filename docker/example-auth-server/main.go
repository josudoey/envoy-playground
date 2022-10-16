package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

const (
	grpcKeepaliveTime        = 30 * time.Second
	grpcKeepaliveTimeout     = 5 * time.Second
	grpcKeepaliveMinTime     = 30 * time.Second
	grpcMaxConcurrentStreams = 1000000
)

var _ auth.AuthorizationServer = (*PureAuthHandler)(nil)

type PureAuthHandler struct{}

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
				Headers: []*core.HeaderValueOption{
					{Header: &core.HeaderValue{Key: "WWW-Authenticate", Value: `Basic realm="Access to upstream", charset="UTF-8"`}},
				},
				Body: "Unauthorized(default user&password: guest)",
			},
		},
	}
)

func getPlustHeaders(headers map[string]string) []*core.HeaderValueOption {
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

func (s *PureAuthHandler) Check(ctx context.Context, req *auth.CheckRequest) (*auth.CheckResponse, error) {
	// see https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/header/v3/external_auth.proto
	if headerAuth, exists := req.Attributes.Request.Http.Headers["authorization"]; !exists {
		return CheckUnauthorized, nil
	} else if headerAuth != "Basic Z3Vlc3Q6Z3Vlc3Q=" {
		return CheckUnauthorized, nil
	}

	return &auth.CheckResponse{
		Status: &status.Status{
			Code: int32(code.Code_OK),
		},
		HttpResponse: &auth.CheckResponse_OkResponse{
			OkResponse: &auth.OkHttpResponse{
				Headers: getPlustHeaders(req.Attributes.Request.Http.Headers),
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

	// gRPC golang library sets a very small upper bound for the number gRPC/h2
	// streams over a single TCP connection. If a proxy multiplexes requests over
	// a single connection to the management server, then it might lead to
	// availability problems. Keepalive timeouts based on connection_keepalive parameter https://www.envoyproxy.io/docs/envoy/latest/configuration/overview/examples#dynamic
	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions,
		grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    grpcKeepaliveTime,
			Timeout: grpcKeepaliveTimeout,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             grpcKeepaliveMinTime,
			PermitWithoutStream: true,
		}),
	)
	grpcServer := grpc.NewServer(grpcOptions...)
	auth.RegisterAuthorizationServer(grpcServer, &PureAuthHandler{})

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("auth server listening on %d\n", port)
	if err = grpcServer.Serve(lis); err != nil {
		log.Println(err)
	}
}
