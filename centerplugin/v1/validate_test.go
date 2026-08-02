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
	request := &ActivateRequest{Permissions: []string{PermissionNodesRead}}
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
