// Package pluginfixture implements the non-production plugin used by Relayward contract and host tests.
package pluginfixture

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	centerpluginv1 "github.com/Relayward/relayward-sdk/centerplugin/v1"
	"github.com/Relayward/relayward-sdk/contract"
	nodepluginv1 "github.com/Relayward/relayward-sdk/nodeplugin/v1"
)

const inheritedSecretSentinel = "RELAYWARD_TEST_SECRET"

func Run(version string) int {
	if version == "" {
		return 2
	}
	if os.Getenv(inheritedSecretSentinel) != "" || os.Getenv("RELAYWARD_REGISTRATION_TOKEN") != "" {
		return 41
	}
	if socket := os.Getenv(centerpluginv1.EnvironmentPluginSocket); socket != "" {
		return runCenter(socket, version)
	}
	if socket := os.Getenv(nodepluginv1.EnvironmentSocketPath); socket != "" {
		return runNode(socket, version)
	}
	return 2
}

type centerPlugin struct {
	centerpluginv1.UnimplementedCenterPluginServer
	pluginID string
	version  string
	mu       sync.Mutex
	active   bool
	host     *grpc.ClientConn
}

func runCenter(socket, version string) int {
	listener, err := privateListener(socket)
	if err != nil {
		return 42
	}
	defer listener.Close()
	server := grpc.NewServer()
	centerpluginv1.RegisterCenterPluginServer(server, &centerPlugin{
		pluginID: os.Getenv(centerpluginv1.EnvironmentPluginID), version: version,
	})
	if err := server.Serve(listener); err != nil {
		return 44
	}
	return 0
}

func (plugin *centerPlugin) GetInfo(context.Context, *centerpluginv1.GetInfoRequest) (*centerpluginv1.GetInfoResponse, error) {
	return &centerpluginv1.GetInfoResponse{
		ApiVersion: contract.CenterPluginAPIVersion, PluginId: plugin.pluginID, Version: plugin.version,
	}, nil
}

func (plugin *centerPlugin) Activate(ctx context.Context, request *centerpluginv1.ActivateRequest) (*centerpluginv1.Activated, error) {
	if err := centerpluginv1.ValidateActivateRequest(request); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid activation")
	}
	permissions := append([]string(nil), request.Permissions...)
	sort.Strings(permissions)
	connection, err := grpc.NewClient("passthrough:///relayward-contract-host",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(dialContext context.Context, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(dialContext, "unix", os.Getenv(centerpluginv1.EnvironmentHostSocket))
		}),
	)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "plugin Host is unavailable")
	}
	if contains(permissions, centerpluginv1.PermissionNodesRead) {
		if _, err := centerpluginv1.NewPluginHostClient(connection).ListNodes(ctx, &centerpluginv1.ListNodesRequest{}); err != nil {
			connection.Close()
			return nil, err
		}
	}
	plugin.mu.Lock()
	if plugin.host != nil {
		_ = plugin.host.Close()
	}
	plugin.host = connection
	plugin.active = true
	plugin.mu.Unlock()
	return &centerpluginv1.Activated{Permissions: permissions}, nil
}

func (plugin *centerPlugin) GetStatus(context.Context, *centerpluginv1.GetStatusRequest) (*centerpluginv1.GetStatusResponse, error) {
	plugin.mu.Lock()
	active := plugin.active
	plugin.mu.Unlock()
	health := centerpluginv1.Health_HEALTH_STARTING
	if active {
		health = centerpluginv1.Health_HEALTH_HEALTHY
	}
	return &centerpluginv1.GetStatusResponse{Health: health}, nil
}

func (plugin *centerPlugin) InvokeUI(ctx context.Context, request *centerpluginv1.InvokeUIRequest) (*centerpluginv1.InvokeUIResponse, error) {
	if err := centerpluginv1.ValidateInvokeUIRequest(request); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid UI request")
	}
	plugin.mu.Lock()
	host := plugin.host
	plugin.mu.Unlock()
	if host == nil {
		return nil, status.Error(codes.Unavailable, "plugin Host is unavailable")
	}
	switch request.Method {
	case "nodes.summary":
		nodes, err := centerpluginv1.NewPluginHostClient(host).ListNodes(ctx, &centerpluginv1.ListNodesRequest{})
		if err != nil {
			return nil, err
		}
		raw, _ := json.Marshal(map[string]int{"count": len(nodes.Nodes)})
		return &centerpluginv1.InvokeUIResponse{Json: raw}, nil
	case "status.read":
		raw, _ := json.Marshal(map[string]string{"plugin_id": plugin.pluginID, "version": plugin.version})
		return &centerpluginv1.InvokeUIResponse{Json: raw}, nil
	case "services.replace":
		var input struct {
			NodeID string `json:"node_id"`
		}
		if err := json.Unmarshal(request.Json, &input); err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid service replacement")
		}
		response, err := centerpluginv1.NewPluginHostClient(host).ReplaceServices(ctx, &centerpluginv1.ReplaceServicesRequest{
			NodeId: input.NodeID,
			Services: []*centerpluginv1.PluginService{{
				Id: "contract-main", DisplayName: "Contract service", Enabled: true,
				Capabilities: []string{"subscription.render"}, SubscriptionSha256: strings.Repeat("a", 64),
			}},
		})
		if err != nil {
			return nil, err
		}
		raw, _ := json.Marshal(map[string]uint32{"service_count": response.ServiceCount})
		return &centerpluginv1.InvokeUIResponse{Json: raw}, nil
	case "events.publish":
		var input struct {
			NodeID             string `json:"node_id"`
			SourceEventID      string `json:"source_event_id"`
			ObservedAtUnixNano int64  `json:"observed_at_unix_nano"`
		}
		if err := json.Unmarshal(request.Json, &input); err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid event publication")
		}
		response, err := centerpluginv1.NewPluginHostClient(host).PublishEvents(ctx, &centerpluginv1.PublishEventsRequest{
			Events: []*centerpluginv1.PublishedEvent{{
				SourceEventId: input.SourceEventID, NodeId: input.NodeID,
				Kind: centerpluginv1.EventNotificationRequest, ObservedAtUnixNano: input.ObservedAtUnixNano,
				Json: []byte(`{"severity":"info","subject":"Contract event","body":"The contract plugin published a test event."}`),
			}},
		})
		if err != nil {
			return nil, err
		}
		raw, _ := json.Marshal(map[string]uint32{"event_count": response.EventCount})
		return &centerpluginv1.InvokeUIResponse{Json: raw}, nil
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported UI method")
	}
}

func (plugin *centerPlugin) RenderSubscription(_ context.Context, request *centerpluginv1.RenderSubscriptionRequest) (*centerpluginv1.RenderSubscriptionResponse, error) {
	if err := centerpluginv1.ValidateRenderSubscriptionRequest(request); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid subscription request")
	}
	response := &centerpluginv1.RenderSubscriptionResponse{Services: make([]*centerpluginv1.SubscriptionServiceContribution, len(request.Services))}
	for index, service := range request.Services {
		name := service.DisplayName + " / Contract"
		uri := (&url.URL{
			Scheme: "relayward-test", Host: "fixture.example.com", Path: "/" + service.ServiceId,
			User: url.User(request.AuthorizationId), Fragment: name,
		}).String()
		mihomo, _ := json.Marshal(map[string]any{
			"name": name, "type": "relayward-test", "server": "fixture.example.com",
			"authorization_id": request.AuthorizationId, "service_id": service.ServiceId,
		})
		singBox, _ := json.Marshal(map[string]any{
			"tag": name, "type": "relayward-test", "server": "fixture.example.com",
			"authorization_id": request.AuthorizationId, "service_id": service.ServiceId,
		})
		response.Services[index] = &centerpluginv1.SubscriptionServiceContribution{
			ServiceId: service.ServiceId, DisplayName: name, Uris: []string{uri},
			MihomoProxiesJson: [][]byte{mihomo}, SingBoxOutboundsJson: [][]byte{singBox},
		}
	}
	if err := centerpluginv1.ValidateRenderSubscriptionResponse(request, response); err != nil {
		return nil, status.Error(codes.Internal, "generated subscription is invalid")
	}
	return response, nil
}

func (*centerPlugin) ConsumeEvents(_ context.Context, request *centerpluginv1.ConsumeEventsRequest) (*centerpluginv1.EventsConsumed, error) {
	if err := centerpluginv1.ValidateConsumeEventsRequest(request); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid event batch")
	}
	response := &centerpluginv1.EventsConsumed{ThroughCursor: request.Events[len(request.Events)-1].Cursor}
	if err := centerpluginv1.ValidateEventsConsumed(request, response); err != nil {
		return nil, status.Error(codes.Internal, "generated event acknowledgement is invalid")
	}
	return response, nil
}

type nodePlugin struct {
	nodepluginv1.UnimplementedNodePluginServer
	pluginID    string
	version     string
	mu          sync.Mutex
	generation  uint64
	digest      string
	degrade     bool
	statusCalls int
}

func runNode(socket, version string) int {
	listener, err := privateListener(socket)
	if err != nil {
		return 42
	}
	defer listener.Close()
	server := grpc.NewServer()
	nodepluginv1.RegisterNodePluginServer(server, &nodePlugin{
		pluginID: os.Getenv(nodepluginv1.EnvironmentPluginID), version: version,
	})
	if err := server.Serve(listener); err != nil {
		return 44
	}
	return 0
}

func (plugin *nodePlugin) GetInfo(context.Context, *nodepluginv1.GetInfoRequest) (*nodepluginv1.GetInfoResponse, error) {
	return &nodepluginv1.GetInfoResponse{
		ApiVersion: contract.NodePluginAPIVersion, PluginId: plugin.pluginID, Version: plugin.version,
		Capabilities: []string{nodepluginv1.CapabilityRecentActivity, nodepluginv1.CapabilityDynamicBlocking,
			nodepluginv1.CapabilityServiceControl, nodepluginv1.CapabilityTrafficCounters},
		TelemetryStreamId: "0123456789abcdef0123456789abcdef",
	}, nil
}

func (*nodePlugin) CollectTelemetry(_ context.Context, request *nodepluginv1.CollectTelemetryRequest) (*nodepluginv1.CollectTelemetryResponse, error) {
	if err := nodepluginv1.ValidateCollectTelemetryRequest(request); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid telemetry request")
	}
	return &nodepluginv1.CollectTelemetryResponse{ObservedAtUnixNano: time.Now().UTC().UnixNano(), NextSequence: request.AfterSequence}, nil
}

func (*nodePlugin) SetServiceState(_ context.Context, request *nodepluginv1.SetServiceStateRequest) (*nodepluginv1.SetServiceStateResponse, error) {
	if err := nodepluginv1.ValidateSetServiceStateRequest(request); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid service state")
	}
	return &nodepluginv1.SetServiceStateResponse{PolicyGeneration: request.PolicyGeneration, StateRevision: request.StateRevision, AuthorizationId: request.AuthorizationId,
		ServiceId: request.ServiceId, Enabled: request.Enabled, Reason: request.Reason}, nil
}

func (*nodePlugin) ReplaceDynamicBlocks(_ context.Context, request *nodepluginv1.ReplaceDynamicBlocksRequest) (*nodepluginv1.ReplaceDynamicBlocksResponse, error) {
	if err := nodepluginv1.ValidateReplaceDynamicBlocksRequest(request); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid dynamic block set")
	}
	return &nodepluginv1.ReplaceDynamicBlocksResponse{PolicyGeneration: request.PolicyGeneration,
		BlockRevision: request.BlockRevision, BlockCount: uint32(len(request.Blocks))}, nil
}

func (*nodePlugin) ValidateConfiguration(_ context.Context, request *nodepluginv1.ConfigurationRequest) (*nodepluginv1.ConfigurationValidated, error) {
	if err := nodepluginv1.ValidateConfigurationRequest(request); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid configuration")
	}
	if bytes.Contains(request.Json, []byte(`"reject":true`)) {
		return nil, status.Error(codes.InvalidArgument, "configuration rejected")
	}
	return &nodepluginv1.ConfigurationValidated{Generation: request.Generation, Sha256: request.Sha256}, nil
}

func (plugin *nodePlugin) ApplyConfiguration(_ context.Context, request *nodepluginv1.ConfigurationRequest) (*nodepluginv1.ConfigurationApplied, error) {
	if err := nodepluginv1.ValidateConfigurationRequest(request); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid configuration")
	}
	plugin.mu.Lock()
	plugin.generation = request.Generation
	plugin.digest = request.Sha256
	plugin.degrade = bytes.Contains(request.Json, []byte(`"degrade":true`))
	plugin.statusCalls = 0
	plugin.mu.Unlock()
	return &nodepluginv1.ConfigurationApplied{Generation: request.Generation, Sha256: request.Sha256}, nil
}

func (plugin *nodePlugin) GetStatus(context.Context, *nodepluginv1.GetStatusRequest) (*nodepluginv1.GetStatusResponse, error) {
	plugin.mu.Lock()
	defer plugin.mu.Unlock()
	health := nodepluginv1.Health_HEALTH_STARTING
	if plugin.generation != 0 {
		health = nodepluginv1.Health_HEALTH_HEALTHY
		if plugin.degrade && plugin.statusCalls > 0 {
			health = nodepluginv1.Health_HEALTH_UNHEALTHY
		}
		plugin.statusCalls++
	}
	return &nodepluginv1.GetStatusResponse{
		Generation: plugin.generation, ConfigurationSha256: plugin.digest, Health: health,
	}, nil
}

func privateListener(path string) (net.Listener, error) {
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
