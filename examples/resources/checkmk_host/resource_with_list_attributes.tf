# Example: host attributes that CheckMK types as lists
#
# `attributes` is a map of strings, so the attributes CheckMK declares as arrays
# (parents, additional_ipv4addresses, additional_ipv6addresses) cannot go in
# there - the API rejects a string with "Not a valid list.". They have their own
# typed resource attributes instead.

resource "checkmk_host" "hypervisor" {
  host_name = "hypervisor-01"
  folder    = "/"

  attributes = {
    alias     = "Hypervisor 01"
    ipaddress = "10.0.1.10"
  }
}

resource "checkmk_host" "guest" {
  host_name = "guest-01"
  folder    = "/"

  # Reachability of the guest depends on the hypervisor: CheckMK marks the guest
  # UNREACHABLE instead of DOWN while the parent is down.
  parents = [checkmk_host.hypervisor.host_name]

  additional_ipv4addresses = ["10.0.2.11"]

  attributes = {
    alias     = "Guest 01"
    ipaddress = "10.0.2.10"
  }
}
