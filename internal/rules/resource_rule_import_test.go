package rules

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/withakedo/terraform-provider-checkmk/internal/client"
	"github.com/withakedo/terraform-provider-checkmk/internal/common"
)

const importedRuleID = "0123456789abcdef0123456789abcdef"

const importedRuleBody = `{
	"id": "0123456789abcdef0123456789abcdef",
	"title": "APT updates",
	"domainType": "rule",
	"extensions": {
		"ruleset": "checkgroup_parameters:apt",
		"folder": "/",
		"properties": {
			"description": "APT updates",
			"comment": "managed by terraform",
			"disabled": false
		},
		"value_raw": "{'normal': 0, 'security': 1}",
		"conditions": {
			"host_name": {
				"match_on": ["ignored-host"],
				"operator": "none_of"
			}
		}
	}
}`

// TestRuleResourceImportThenRead covers importing a generic checkmk_rule by its
// CheckMK UUID: the state after ImportState holds a null properties object, and
// Read must populate it from the API instead of failing to decode it.
func TestRuleResourceImportThenRead(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/check_mk/api/1.0/version" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"versions": {"checkmk": "2.5.0p12"}}`))
			return
		}
		if r.URL.Path != "/check_mk/api/1.0/objects/rule/"+importedRuleID {
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", `"abc"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(importedRuleBody))
	}))
	defer server.Close()

	apiClient, err := client.NewClient(server.URL, "automation", "secret")
	if err != nil {
		t.Fatalf("NewClient: %s", err)
	}

	r := &RuleResource{providerData: &common.ProviderData{Client: apiClient}}

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema: %s", schemaResp.Diagnostics)
	}
	ruleSchema := schemaResp.Schema
	emptyState := tfsdk.State{
		Schema: ruleSchema,
		Raw:    tftypes.NewValue(ruleSchema.Type().TerraformType(ctx), nil),
	}

	importResp := &resource.ImportStateResponse{State: emptyState}
	r.ImportState(ctx, resource.ImportStateRequest{ID: importedRuleID}, importResp)
	if importResp.Diagnostics.HasError() {
		t.Fatalf("ImportState: %s", importResp.Diagnostics)
	}

	// The imported state must indeed carry a null properties object; otherwise
	// this test would not exercise the regression at all.
	var imported RuleResourceModel
	if diags := importResp.State.Get(ctx, &imported); diags.HasError() {
		t.Fatalf("reading imported state: %s", diags)
	}
	if !imported.Properties.IsNull() {
		t.Fatalf("expected properties to be null after import, got %v", imported.Properties)
	}

	readResp := &resource.ReadResponse{State: tfsdk.State{Schema: ruleSchema, Raw: importResp.State.Raw}}
	r.Read(ctx, resource.ReadRequest{State: importResp.State}, readResp)
	if readResp.Diagnostics.HasError() {
		t.Fatalf("Read: %s", readResp.Diagnostics)
	}

	var data RuleResourceModel
	if diags := readResp.State.Get(ctx, &data); diags.HasError() {
		t.Fatalf("reading state: %s", diags)
	}

	if got := data.Ruleset.ValueString(); got != "checkgroup_parameters:apt" {
		t.Errorf("ruleset = %q", got)
	}
	if got := data.ValueRaw.ValueString(); got != "{'normal': 0, 'security': 1}" {
		t.Errorf("value_raw = %q", got)
	}

	if data.Properties.IsNull() || data.Properties.IsUnknown() {
		t.Fatalf("properties not populated: %v", data.Properties)
	}
	var props RulePropertiesModel
	if diags := data.Properties.As(ctx, &props, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("decoding properties: %s", diags)
	}
	if got := props.Description.ValueString(); got != "APT updates" {
		t.Errorf("properties.description = %q", got)
	}
	if got := props.Comment.ValueString(); got != "managed by terraform" {
		t.Errorf("properties.comment = %q", got)
	}
	if props.Disabled.ValueBool() {
		t.Errorf("properties.disabled = true, want false")
	}

	if data.Conditions.IsNull() || data.Conditions.IsUnknown() {
		t.Fatalf("conditions not populated: %v", data.Conditions)
	}
	hostName, ok := data.Conditions.Attributes()["host_name"].(types.Object)
	if !ok || hostName.IsNull() {
		t.Fatalf("conditions.host_name not populated: %v", data.Conditions.Attributes()["host_name"])
	}
	operator, ok := hostName.Attributes()["operator"].(types.String)
	if !ok || operator.ValueString() != "none_of" {
		t.Errorf("conditions.host_name.operator = %v", hostName.Attributes()["operator"])
	}
}
