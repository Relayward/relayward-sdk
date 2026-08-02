package centerpluginv1

import (
	"testing"

	"github.com/Relayward/relayward-sdk/contract"
)

func TestCenterPluginLifecycleValidation(t *testing.T) {
	info := &GetInfoResponse{ApiVersion: contract.CenterPluginAPIVersion, PluginId: "io.relayward.test", Version: "1.2.3"}
	if err := ValidateInfoResponse(info, info.PluginId, info.Version); err != nil {
		t.Fatalf("ValidateInfoResponse() error = %v", err)
	}
	request := &ActivateRequest{Permissions: []string{PermissionNodesRead, PermissionServicesWrite}}
	if err := ValidateActivated(request, &Activated{Permissions: append([]string(nil), request.Permissions...)}); err != nil {
		t.Fatalf("ValidateActivated() error = %v", err)
	}
	if err := ValidateStatusResponse(&GetStatusResponse{Health: Health_HEALTH_HEALTHY}); err != nil {
		t.Fatalf("ValidateStatusResponse() error = %v", err)
	}
}

func TestCenterPluginValidationRejectsPermissionAndUIBoundaryViolations(t *testing.T) {
	for _, permissions := range [][]string{{"core.users.read"}, {PermissionNodesRead, PermissionNodesRead}} {
		if err := ValidateActivateRequest(&ActivateRequest{Permissions: permissions}); err == nil {
			t.Fatalf("ValidateActivateRequest(%v) succeeded", permissions)
		}
	}
	for _, request := range []*InvokeUIRequest{
		{Method: "../nodes", Json: []byte(`{}`)},
		{Method: "nodes.list", Json: []byte(`[]`)},
		{Method: "nodes.list", Json: []byte(`{"unknown":`)},
	} {
		if err := ValidateInvokeUIRequest(request); err == nil {
			t.Fatalf("ValidateInvokeUIRequest(%+v) succeeded", request)
		}
	}
}

func TestValidateListNodesResponse(t *testing.T) {
	value := &ListNodesResponse{Nodes: []*Node{{Id: "node-1", Name: "Edge", Enabled: true, Connected: true}}}
	if err := ValidateListNodesResponse(value); err != nil {
		t.Fatalf("ValidateListNodesResponse() error = %v", err)
	}
	value.Nodes = append(value.Nodes, value.Nodes[0])
	if err := ValidateListNodesResponse(value); err == nil {
		t.Fatal("ValidateListNodesResponse() accepted a duplicate node")
	}
}

func TestValidateServiceCatalog(t *testing.T) {
	request := &ReplaceServicesRequest{
		NodeId:   "10000000-0000-4000-8000-000000000001",
		Services: []*PluginService{{Id: "vless-main", DisplayName: "VLESS", Enabled: true, Capabilities: []string{"subscription.render"}}},
	}
	if err := ValidateServicesReplaced(request, &ServicesReplaced{ServiceCount: 1}); err != nil {
		t.Fatalf("ValidateServicesReplaced() error = %v", err)
	}
	request.Services = append(request.Services, request.Services[0])
	if err := ValidateReplaceServicesRequest(request); err == nil {
		t.Fatal("ValidateReplaceServicesRequest() accepted a duplicate service")
	}
}

func TestValidateSubscriptionRendering(t *testing.T) {
	request := &RenderSubscriptionRequest{
		AuthorizationId: "20000000-0000-4000-8000-000000000002",
		NodeId:          "10000000-0000-4000-8000-000000000001",
		PublicAddress:   "edge.example.com",
		Services:        []*SubscriptionServiceBinding{{ServiceId: "vless-main", DisplayName: "VLESS"}},
	}
	response := &RenderSubscriptionResponse{Services: []*SubscriptionServiceContribution{{
		ServiceId: request.Services[0].ServiceId, DisplayName: "VLESS / Edge",
		Uris:                 []string{"vless://credential@example.com:443?security=tls#Edge"},
		MihomoProxiesJson:    [][]byte{[]byte(`{"name":"Edge","type":"vless","server":"example.com","port":443}`)},
		SingBoxOutboundsJson: [][]byte{[]byte(`{"tag":"Edge","type":"vless","server":"example.com","server_port":443}`)},
	}}}
	if err := ValidateRenderSubscriptionResponse(request, response); err != nil {
		t.Fatalf("ValidateRenderSubscriptionResponse() error = %v", err)
	}

	response.Services[0].Uris = []string{"not a URI"}
	if err := ValidateRenderSubscriptionResponse(request, response); err == nil {
		t.Fatal("ValidateRenderSubscriptionResponse() accepted an invalid URI")
	}
	response.Services[0].Uris = []string{"vless://credential@example.com:443"}
	response.Services[0].MihomoProxiesJson = [][]byte{[]byte(`[]`)}
	if err := ValidateRenderSubscriptionResponse(request, response); err == nil {
		t.Fatal("ValidateRenderSubscriptionResponse() accepted a non-object fragment")
	}
}
