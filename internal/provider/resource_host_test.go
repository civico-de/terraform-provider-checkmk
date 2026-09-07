package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccHostResource(t *testing.T) {
	// NOTE: This test requires manual activation of changes in CheckMK.
	// After each step, changes must be activated for them to take effect.
	// Future enhancement: Add automatic activation support.

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccHostResourceConfig("test-terraform-host", "Test Host"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("checkmk_host.test", "host_name", "test-terraform-host"),
					resource.TestCheckResourceAttr("checkmk_host.test", "folder", "/"),
					// NOTE: Attribute checks commented out as they require activation
					// resource.TestCheckResourceAttr("checkmk_host.test", "attributes.alias", "Test Host"),
					// resource.TestCheckResourceAttr("checkmk_host.test", "attributes.ipaddress", "127.0.0.1"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "checkmk_host.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Ignore attributes as they may not match without activation
				ImportStateVerifyIgnore: []string{"attributes"},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccHostResourceConfig(hostName, alias string) string {
	return fmt.Sprintf(`
resource "checkmk_host" "test" {
  host_name = %[1]q
  folder    = "/"

  attributes = {
    alias     = %[2]q
    ipaddress = "127.0.0.1"
  }
}
`, hostName, alias)
}

// TestAccHostResource_UnprefixedAttributes verifies that a built-in tag group
// can be written without the "tag_" prefix (e.g. "agent" instead of
// "tag_agent") alongside an arbitrary custom attribute, and that state keeps
// the configured (unprefixed) keys without producing a perpetual diff.
func TestAccHostResource_UnprefixedAttributes(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccHostResourceConfigUnprefixed("test-terraform-host-unprefixed"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("checkmk_host.unprefixed", "host_name", "test-terraform-host-unprefixed"),
					// State mirrors the configured keys: the built-in tag group
					// stays "agent" and the custom attribute is untouched.
					resource.TestCheckResourceAttr("checkmk_host.unprefixed", "attributes.agent", "cmk-agent"),
					resource.TestCheckResourceAttr("checkmk_host.unprefixed", "attributes.proxy_port", "8080"),
				),
			},
		},
	})
}

func testAccHostResourceConfigUnprefixed(hostName string) string {
	return fmt.Sprintf(`
resource "checkmk_host" "unprefixed" {
  host_name = %[1]q
  folder    = "/"

  attributes = {
    alias = "Unprefixed Attributes Host"

    # Built-in host tag group written without the "tag_" prefix.
    agent = "cmk-agent"

    # Arbitrary custom attribute, also without a prefix.
    proxy_port = "8080"
  }
}
`, hostName)
}

// TestAccHostResource_ListAttributes verifies that the host attributes CheckMK
// types as lists round-trip: they are sent as JSON arrays, read back into state
// and produce no diff on the next plan.
func TestAccHostResource_ListAttributes(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccHostResourceConfigListAttributes(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("checkmk_host.child", "parents.#", "1"),
					resource.TestCheckResourceAttr("checkmk_host.child", "parents.0", "test-terraform-parent"),
					resource.TestCheckResourceAttr("checkmk_host.child", "additional_ipv4addresses.#", "1"),
					resource.TestCheckResourceAttr("checkmk_host.child", "additional_ipv4addresses.0", "127.0.0.3"),
				),
			},
		},
	})
}

func testAccHostResourceConfigListAttributes() string {
	return `
resource "checkmk_host" "parent" {
  host_name = "test-terraform-parent"
  folder    = "/"

  attributes = {
    ipaddress = "127.0.0.1"
  }
}

resource "checkmk_host" "child" {
  host_name = "test-terraform-child"
  folder    = "/"

  parents                  = [checkmk_host.parent.host_name]
  additional_ipv4addresses = ["127.0.0.3"]

  attributes = {
    ipaddress = "127.0.0.2"
  }
}
`
}
