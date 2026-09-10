package hosts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/withakedo/terraform-provider-checkmk/internal/client"
)

func listOf(t *testing.T, values ...string) types.List {
	t.Helper()
	list, diags := types.ListValueFrom(context.Background(), types.StringType, values)
	if diags.HasError() {
		t.Fatalf("failed to build list: %v", diags)
	}
	return list
}

// TestCreateHostSendsListAttributesAsJSONArrays checks that a host created with
// list attributes sends them as JSON arrays. CheckMK answers a string with
// {"attributes": {"parents": ["Not a valid list."]}}.
func TestCreateHostSendsListAttributesAsJSONArrays(t *testing.T) {
	data := &HostResourceModel{
		HostName:                types.StringValue("guest"),
		Parents:                 listOf(t, "hypervisor"),
		AdditionalIPv4Addresses: listOf(t, "10.0.0.2", "10.0.0.3"),
	}

	attributes := map[string]interface{}{"alias": "Guest"}
	if diags := addListAttributes(context.Background(), data, attributes); diags.HasError() {
		t.Fatalf("addListAttributes: %v", diags)
	}

	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"guest","title":"guest","extensions":{"folder":"/","attributes":{}}}`))
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse server URL: %v", err)
	}
	c := &client.Client{HTTPClient: server.Client(), BaseURL: baseURL}

	if _, err := c.CreateHost(context.Background(), &client.HostCreateRequest{
		HostName:   "guest",
		Folder:     "/",
		Attributes: attributes,
	}); err != nil {
		t.Fatalf("CreateHost: %v", err)
	}

	sent, ok := body["attributes"].(map[string]interface{})
	if !ok {
		t.Fatalf("request body has no attributes object: %v", body)
	}
	if got, want := sent["parents"], []interface{}{"hypervisor"}; !reflect.DeepEqual(got, want) {
		t.Errorf("parents sent as %#v, want %#v", got, want)
	}
	if got, want := sent["additional_ipv4addresses"], []interface{}{"10.0.0.2", "10.0.0.3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("additional_ipv4addresses sent as %#v, want %#v", got, want)
	}
	if got, want := sent["alias"], "Guest"; got != want {
		t.Errorf("alias sent as %#v, want %#v", got, want)
	}
	if _, present := sent["additional_ipv6addresses"]; present {
		t.Error("additional_ipv6addresses was sent although it is null")
	}
}

// TestReadHostMapsListAttributesBack checks that the arrays a read returns land
// in the typed resource attributes.
func TestReadHostMapsListAttributesBack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "guest",
			"title": "guest",
			"extensions": {
				"folder": "/",
				"attributes": {
					"alias": "Guest",
					"parents": ["hypervisor"],
					"additional_ipv4addresses": ["10.0.0.2", "10.0.0.3"]
				}
			}
		}`))
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse server URL: %v", err)
	}
	c := &client.Client{HTTPClient: server.Client(), BaseURL: baseURL}

	host, err := c.GetHost(context.Background(), "guest")
	if err != nil {
		t.Fatalf("GetHost: %v", err)
	}

	data := &HostResourceModel{HostName: types.StringValue("guest")}
	if diags := readListAttributes(context.Background(), data, host.Extensions.Attributes); diags.HasError() {
		t.Fatalf("readListAttributes: %v", diags)
	}

	if got, want := data.Parents, listOf(t, "hypervisor"); !got.Equal(want) {
		t.Errorf("parents read as %s, want %s", got, want)
	}
	if got, want := data.AdditionalIPv4Addresses, listOf(t, "10.0.0.2", "10.0.0.3"); !got.Equal(want) {
		t.Errorf("additional_ipv4addresses read as %s, want %s", got, want)
	}
	if !data.AdditionalIPv6Addresses.IsNull() {
		t.Errorf("additional_ipv6addresses read as %s, want null", data.AdditionalIPv6Addresses)
	}
}

// TestReadListAttributesDropsRemovedValues checks that a managed list the API no
// longer reports becomes null, so the next plan shows the drift.
func TestReadListAttributesDropsRemovedValues(t *testing.T) {
	data := &HostResourceModel{Parents: listOf(t, "hypervisor")}

	if diags := readListAttributes(context.Background(), data, map[string]interface{}{}); diags.HasError() {
		t.Fatalf("readListAttributes: %v", diags)
	}

	if !data.Parents.IsNull() {
		t.Errorf("parents read as %s, want null", data.Parents)
	}
}
