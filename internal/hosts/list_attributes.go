package hosts

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// hostListAttributes are the host attributes the CheckMK API declares as arrays
// of strings. They cannot travel in the `attributes` map, which is a map of
// strings, so each gets its own typed resource attribute.
func (m *HostResourceModel) hostListAttributes() map[string]*types.List {
	return map[string]*types.List{
		"parents":                  &m.Parents,
		"additional_ipv4addresses": &m.AdditionalIPv4Addresses,
		"additional_ipv6addresses": &m.AdditionalIPv6Addresses,
	}
}

// addListAttributes writes the configured list attributes into the attribute map
// sent to the API, as JSON arrays. A null attribute is omitted, which clears it
// on update because the API replaces attributes wholesale.
func addListAttributes(ctx context.Context, m *HostResourceModel, attributes map[string]interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	for apiKey, field := range m.hostListAttributes() {
		if field.IsNull() || field.IsUnknown() {
			continue
		}
		var values []string
		diags.Append(field.ElementsAs(ctx, &values, false)...)
		if diags.HasError() {
			return diags
		}
		if values == nil {
			values = []string{}
		}
		attributes[apiKey] = values
	}

	return diags
}

// readListAttributes maps the list attributes of an API response back into the
// model. An attribute the API reports as empty is left as it is in state, so an
// unmanaged attribute does not appear as an empty list.
func readListAttributes(ctx context.Context, m *HostResourceModel, apiAttributes map[string]interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	for apiKey, field := range m.hostListAttributes() {
		values := stringSlice(apiAttributes[apiKey])
		if len(values) == 0 {
			if !field.IsNull() && len(field.Elements()) > 0 {
				// Managed values disappeared upstream - surface the drift.
				*field = types.ListNull(types.StringType)
			}
			continue
		}

		list, d := types.ListValueFrom(ctx, types.StringType, values)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		*field = list
	}

	return diags
}

// stringSlice converts a decoded JSON array of strings into []string.
func stringSlice(value interface{}) []string {
	raw, ok := value.([]interface{})
	if !ok {
		return nil
	}
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		str, ok := item.(string)
		if !ok {
			return nil
		}
		values = append(values, str)
	}
	return values
}
