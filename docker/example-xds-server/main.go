package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	ext_authz "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_authz/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	runtimeservice "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	envoylog "github.com/envoyproxy/go-control-plane/pkg/log"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/envoyproxy/go-control-plane/pkg/test/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

func mustNewAnypb(src protoreflect.ProtoMessage) *anypb.Any {
	any, err := anypb.New(src)
	if err != nil {
		panic(err)
	}
	return any
}

var (
	ExampleUpstreamHttpRoute = &route.Route_Route{Route: &route.RouteAction{
		ClusterSpecifier: &route.RouteAction_Cluster{
			Cluster: "example-upstream-http",
		},
	}}
	ExampleAuthGRPC = mustNewAnypb(&ext_authz.ExtAuthz{
		Services: &ext_authz.ExtAuthz_GrpcService{
			GrpcService: &core.GrpcService{
				TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
						ClusterName: "example-auth-grpc",
					},
				},
			},
		},
		TransportApiVersion: core.ApiVersion_V3,
		StatusOnError:       &typev3.HttpStatus{Code: typev3.StatusCode(typev3.StatusCode_InternalServerError)},
	})
	ExampleAuthFilter = &hcm.HttpFilter{
		Name: wellknown.HTTPExternalAuthorization,
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: ExampleAuthGRPC,
		},
	}
	ExampleLocalRoute *route.RouteConfiguration = &route.RouteConfiguration{
		Name: "example_local_route",
		InternalOnlyHeaders: []string{
			"x-request-id",
		},
		ResponseHeadersToRemove: []string{
			"x-envoy-upstream-service-time",
		},
		VirtualHosts: []*route.VirtualHost{{
			Name:    "example_local_service",
			Domains: []string{"*"},
			Routes: []*route.Route{
				{
					TypedPerFilterConfig: map[string]*anypb.Any{
						ExampleAuthFilter.Name: mustNewAnypb(&ext_authz.ExtAuthzPerRoute{Override: &ext_authz.ExtAuthzPerRoute_CheckSettings{
							CheckSettings: &ext_authz.CheckSettings{
								ContextExtensions: map[string]string{
									"auth-scheme": "bearer",
								},
							},
						}}),
					},
					Match: &route.RouteMatch{
						PathSpecifier: &route.RouteMatch_Prefix{
							Prefix: "/bearer/",
						},
					},
					Action: ExampleUpstreamHttpRoute,
				},
				{
					TypedPerFilterConfig: map[string]*anypb.Any{
						ExampleAuthFilter.Name: mustNewAnypb(&ext_authz.ExtAuthzPerRoute{Override: &ext_authz.ExtAuthzPerRoute_CheckSettings{
							CheckSettings: &ext_authz.CheckSettings{
								ContextExtensions: map[string]string{
									"auth-scheme": "basic",
								},
							},
						}}),
					},
					Match: &route.RouteMatch{
						PathSpecifier: &route.RouteMatch_Prefix{
							Prefix: "/basic/",
						},
					},
					Action: ExampleUpstreamHttpRoute,
				}, {
					TypedPerFilterConfig: map[string]*anypb.Any{
						ExampleAuthFilter.Name: mustNewAnypb(&ext_authz.ExtAuthzPerRoute{Override: &ext_authz.ExtAuthzPerRoute_Disabled{
							Disabled: true,
						}}),
					},
					Match: &route.RouteMatch{
						PathSpecifier: &route.RouteMatch_Prefix{
							Prefix: "/",
						},
					},
					Action: ExampleUpstreamHttpRoute,
				}},
		}},
	}

	ExampleRds = &hcm.Rds{
		RouteConfigName: ExampleLocalRoute.Name,
		ConfigSource: &core.ConfigSource{
			ResourceApiVersion: resource.DefaultAPIVersion,
			ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
				ApiConfigSource: &core.ApiConfigSource{
					TransportApiVersion:       resource.DefaultAPIVersion,
					ApiType:                   core.ApiConfigSource_GRPC,
					SetNodeOnFirstMessageOnly: true,
					GrpcServices: []*core.GrpcService{{
						TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
							EnvoyGrpc: &core.GrpcService_EnvoyGrpc{ClusterName: "example-xds-grpc"},
						},
					}},
				},
			},
		},
	}

	ExampleUpstreamHttpCluster *cluster.Cluster = &cluster.Cluster{
		Name:                 "example-upstream-http",
		ConnectTimeout:       durationpb.New(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_LOGICAL_DNS},
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: "example-upstream-http",
			Endpoints: []*endpoint.LocalityLbEndpoints{{
				LbEndpoints: []*endpoint.LbEndpoint{{
					HostIdentifier: &endpoint.LbEndpoint_Endpoint{
						Endpoint: &endpoint.Endpoint{
							Address: &core.Address{
								Address: &core.Address_SocketAddress{
									SocketAddress: &core.SocketAddress{
										Protocol: core.SocketAddress_TCP,
										Address:  "example-upstream",
										PortSpecifier: &core.SocketAddress_PortValue{
											PortValue: 80,
										},
									},
								},
							},
						},
					},
				}},
			}},
		},
		DnsLookupFamily: cluster.Cluster_V4_ONLY,
	}

	ExampleProxyListener = &listener.Listener{
		Name: "example_proxy_listener",
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.SocketAddress_TCP,
					Address:  "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: 10000,
					},
				},
			},
		},
		FilterChains: []*listener.FilterChain{{
			Filters: []*listener.Filter{
				{
					Name: wellknown.HTTPConnectionManager,
					ConfigType: &listener.Filter_TypedConfig{
						TypedConfig: mustNewAnypb(&hcm.HttpConnectionManager{
							ServerName: "example", // see https://github.com/envoyproxy/envoy/blob/v1.23.1/api/envoy/extensions/filters/network/http_connection_manager/v3/http_connection_manager.proto#L422
							CodecType:  hcm.HttpConnectionManager_AUTO,
							StatPrefix: "http",
							HttpFilters: []*hcm.HttpFilter{
								ExampleAuthFilter,
								{
									Name: wellknown.Router,
									ConfigType: &hcm.HttpFilter_TypedConfig{
										TypedConfig: mustNewAnypb(&routerv3.Router{}),
									},
								}},
							RouteSpecifier: &hcm.HttpConnectionManager_Rds{
								Rds: ExampleRds,
							},
						}),
					},
				}},
		}},
	}
)

func GenerateSnapshot() *cache.Snapshot {
	snap, _ := cache.NewSnapshot("1",
		map[resource.Type][]types.Resource{
			resource.RouteType:    {ExampleLocalRoute},
			resource.ClusterType:  {ExampleUpstreamHttpCluster},
			resource.ListenerType: {ExampleProxyListener},
		},
	)
	return snap
}

func registerServer(grpcServer *grpc.Server, server server.Server) {
	// register services
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, server)
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, server)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, server)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, server)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, server)
	secretservice.RegisterSecretDiscoveryServiceServer(grpcServer, server)
	runtimeservice.RegisterRuntimeDiscoveryServiceServer(grpcServer, server)
}

const (
	grpcKeepaliveTime        = 30 * time.Second
	grpcKeepaliveTimeout     = 5 * time.Second
	grpcKeepaliveMinTime     = 30 * time.Second
	grpcMaxConcurrentStreams = 1000000
)

// RunServer starts an xDS server at the given port.
func RunServer(ctx context.Context, srv server.Server, port uint) {
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
	registerServer(grpcServer, srv)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("management server listening on %d\n", port)
	if err = grpcServer.Serve(lis); err != nil {
		log.Println(err)
	}
}

func main() {
	// see https://github.com/envoyproxy/go-control-plane/blob/v0.10.3/internal/example/main/main.go
	var (
		port   uint
		debug  bool
		nodeID string
	)

	flag.BoolVar(&debug, "debug", false, "Enable xDS server debug logging")

	// The port that this xDS server listens on
	flag.UintVar(&port, "port", 18000, "xDS management server port")

	// Tell Envoy to use this Node ID
	flag.StringVar(&nodeID, "nodeID", "test-id", "Node ID")
	flag.Parse()

	logger := envoylog.NewDefaultLogger()

	sapshotCache := cache.NewSnapshotCache(false, cache.IDHash{}, logger)
	snapshot := GenerateSnapshot()

	if err := snapshot.Consistent(); err != nil {
		logger.Errorf("snapshot inconsistency: %+v\n%+v", snapshot, err)
		os.Exit(1)
	}

	// Add the snapshot to the cache
	if err := sapshotCache.SetSnapshot(context.Background(), nodeID, snapshot); err != nil {
		logger.Errorf("snapshot error %q for %+v", err, snapshot)
		os.Exit(1)
	}

	ctx := context.Background()
	cb := &test.Callbacks{Debug: debug}
	srv := server.NewServer(ctx, sapshotCache, cb)
	RunServer(ctx, srv, port)
}
