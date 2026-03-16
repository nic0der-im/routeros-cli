// Package schema provides JSON Schema definitions for routeros-cli structured
// output types. AI agents can use these schemas to understand the shape of
// JSON responses returned by each command.
package schema

import "sort"

// Schema is a simplified JSON Schema representation. It intentionally omits
// features like $ref, oneOf, or format to keep the definitions portable and
// easy for AI agents to consume.
type Schema struct {
	Type        string            `json:"type"`
	Description string            `json:"description,omitempty"`
	Properties  map[string]Schema `json:"properties,omitempty"`
	Items       *Schema           `json:"items,omitempty"`
	Required    []string          `json:"required,omitempty"`
}

// ---------------------------------------------------------------------------
// Envelope schemas
// ---------------------------------------------------------------------------

// EnvelopeSchema returns the successful response envelope: {ok, data, meta}.
func EnvelopeSchema() Schema {
	return Schema{
		Type:        "object",
		Description: "Successful response envelope",
		Properties: map[string]Schema{
			"ok": {Type: "boolean", Description: "Always true for successful responses"},
			"data": {
				Type:        "array",
				Description: "Array of result objects whose shape depends on the command",
			},
			"meta": {
				Type:        "object",
				Description: "Request metadata",
				Properties: map[string]Schema{
					"device":    {Type: "string", Description: "Target device name"},
					"command":   {Type: "string", Description: "Command that produced this output"},
					"timestamp": {Type: "string", Description: "ISO 8601 timestamp of the response"},
					"count":     {Type: "integer", Description: "Number of items in data"},
				},
				Required: []string{"device", "command", "timestamp", "count"},
			},
		},
		Required: []string{"ok", "data", "meta"},
	}
}

// ErrorSchema returns the error response envelope: {ok, error}.
func ErrorSchema() Schema {
	return Schema{
		Type:        "object",
		Description: "Error response envelope",
		Properties: map[string]Schema{
			"ok": {Type: "boolean", Description: "Always false for error responses"},
			"error": {
				Type:        "object",
				Description: "Error details",
				Properties: map[string]Schema{
					"code":    {Type: "string", Description: "Machine-readable error code"},
					"message": {Type: "string", Description: "Human-readable error message"},
					"device":  {Type: "string", Description: "Target device name"},
				},
				Required: []string{"code", "message", "device"},
			},
		},
		Required: []string{"ok", "error"},
	}
}

// ---------------------------------------------------------------------------
// Command data-payload schemas
//
// Each function returns the schema for a single item in the "data" array.
// All values are typed as "string" because the RouterOS API returns everything
// as strings and the JSON output preserves that representation.
// ---------------------------------------------------------------------------

// SystemInfoSchema returns the schema for "system info" items.
func SystemInfoSchema() Schema {
	return Schema{
		Type:        "object",
		Description: "Combined system identity and resource information",
		Properties: map[string]Schema{
			"Identity":         {Type: "string", Description: "Router identity name"},
			"Board":            {Type: "string", Description: "Board/model name"},
			"Platform":         {Type: "string", Description: "Hardware platform"},
			"Version":          {Type: "string", Description: "RouterOS version"},
			"Uptime":           {Type: "string", Description: "System uptime"},
			"CPU Load":         {Type: "string", Description: "Current CPU load percentage"},
			"Memory Free/Total": {Type: "string", Description: "Free and total memory"},
			"HDD Free/Total":   {Type: "string", Description: "Free and total disk space"},
		},
	}
}

// SystemResourceSchema returns the schema for "system resource" items.
func SystemResourceSchema() Schema {
	return Schema{
		Type:        "object",
		Description: "System resource utilization",
		Properties: map[string]Schema{
			"Board":            {Type: "string", Description: "Board/model name"},
			"Platform":         {Type: "string", Description: "Hardware platform"},
			"Version":          {Type: "string", Description: "RouterOS version"},
			"Uptime":           {Type: "string", Description: "System uptime"},
			"CPU":              {Type: "string", Description: "CPU core count"},
			"CPU Load":         {Type: "string", Description: "Current CPU load percentage"},
			"Memory Free/Total": {Type: "string", Description: "Free and total memory"},
			"HDD Free/Total":   {Type: "string", Description: "Free and total disk space"},
		},
	}
}

// SystemIdentitySchema returns the schema for "system identity" items.
func SystemIdentitySchema() Schema {
	return Schema{
		Type:        "object",
		Description: "System identity",
		Properties: map[string]Schema{
			"Name": {Type: "string", Description: "Router identity name"},
		},
	}
}

// InterfaceSchema returns the schema for "interface" items.
func InterfaceSchema() Schema {
	return Schema{
		Type:        "object",
		Description: "Network interface",
		Properties: map[string]Schema{
			"Name":     {Type: "string", Description: "Interface name"},
			"Type":     {Type: "string", Description: "Interface type (ether, bridge, vlan, etc.)"},
			"MTU":      {Type: "string", Description: "Actual MTU value"},
			"Running":  {Type: "string", Description: "Whether the interface is running (true/false)"},
			"Disabled": {Type: "string", Description: "Whether the interface is disabled (true/false)"},
			"Comment":  {Type: "string", Description: "User-defined comment"},
		},
	}
}

// IPAddressSchema returns the schema for "ip address" items.
func IPAddressSchema() Schema {
	return Schema{
		Type:        "object",
		Description: "IP address assignment",
		Properties: map[string]Schema{
			"Address":   {Type: "string", Description: "IP address with CIDR prefix (e.g. 192.168.1.1/24)"},
			"Network":   {Type: "string", Description: "Network address"},
			"Interface": {Type: "string", Description: "Interface the address is assigned to"},
			"Disabled":  {Type: "string", Description: "Whether the address is disabled (true/false)"},
			"Comment":   {Type: "string", Description: "User-defined comment"},
		},
	}
}

// IPRouteSchema returns the schema for "ip route" items.
func IPRouteSchema() Schema {
	return Schema{
		Type:        "object",
		Description: "IP route entry",
		Properties: map[string]Schema{
			"Dst Address":   {Type: "string", Description: "Destination network (e.g. 0.0.0.0/0)"},
			"Gateway":       {Type: "string", Description: "Next-hop gateway address"},
			"Distance":      {Type: "string", Description: "Administrative distance"},
			"Routing Table": {Type: "string", Description: "Routing table name"},
			"Disabled":      {Type: "string", Description: "Whether the route is disabled (true/false)"},
		},
	}
}

// DNSSettingsSchema returns the schema for "dns" items.
func DNSSettingsSchema() Schema {
	return Schema{
		Type:        "object",
		Description: "DNS resolver settings",
		Properties: map[string]Schema{
			"Servers":        {Type: "string", Description: "Configured DNS servers"},
			"Dynamic Servers": {Type: "string", Description: "Dynamically learned DNS servers"},
			"Allow Remote":   {Type: "string", Description: "Whether remote DNS requests are allowed"},
			"Cache Size":     {Type: "string", Description: "DNS cache size"},
			"Cache Used":     {Type: "string", Description: "DNS cache entries currently in use"},
		},
	}
}

// FirewallRuleSchema returns the schema for "firewall filter" items.
func FirewallRuleSchema() Schema {
	return Schema{
		Type:        "object",
		Description: "Firewall filter rule",
		Properties: map[string]Schema{
			"Chain":       {Type: "string", Description: "Firewall chain (input, forward, output)"},
			"Action":      {Type: "string", Description: "Rule action (accept, drop, reject, etc.)"},
			"Protocol":    {Type: "string", Description: "Protocol (tcp, udp, icmp, etc.)"},
			"Src Address": {Type: "string", Description: "Source address or network"},
			"Dst Address": {Type: "string", Description: "Destination address or network"},
			"Dst Port":    {Type: "string", Description: "Destination port or port range"},
			"Comment":     {Type: "string", Description: "User-defined comment"},
			"Disabled":    {Type: "string", Description: "Whether the rule is disabled (true/false)"},
		},
	}
}

// DHCPLeaseSchema returns the schema for "dhcp lease" items.
func DHCPLeaseSchema() Schema {
	return Schema{
		Type:        "object",
		Description: "DHCP lease entry",
		Properties: map[string]Schema{
			"Address":     {Type: "string", Description: "Leased IP address"},
			"MAC Address": {Type: "string", Description: "Client MAC address"},
			"Host Name":   {Type: "string", Description: "Client hostname"},
			"Status":      {Type: "string", Description: "Lease status (bound, waiting, etc.)"},
			"Comment":     {Type: "string", Description: "User-defined comment"},
		},
	}
}

// DHCPServerSchema returns the schema for "dhcp server" items.
func DHCPServerSchema() Schema {
	return Schema{
		Type:        "object",
		Description: "DHCP server configuration",
		Properties: map[string]Schema{
			"Name":         {Type: "string", Description: "DHCP server name"},
			"Interface":    {Type: "string", Description: "Interface the server listens on"},
			"Address Pool": {Type: "string", Description: "IP pool used for leases"},
			"Lease Time":   {Type: "string", Description: "Default lease duration"},
			"Disabled":     {Type: "string", Description: "Whether the server is disabled (true/false)"},
		},
	}
}

// DHCPPoolSchema returns the schema for "dhcp pool" items.
func DHCPPoolSchema() Schema {
	return Schema{
		Type:        "object",
		Description: "IP address pool",
		Properties: map[string]Schema{
			"Name":    {Type: "string", Description: "Pool name"},
			"Ranges":  {Type: "string", Description: "Address ranges (e.g. 192.168.1.100-192.168.1.200)"},
			"Comment": {Type: "string", Description: "User-defined comment"},
		},
	}
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// registry maps command names to schema constructor functions.
var registry = map[string]func() Schema{
	"system-info":     SystemInfoSchema,
	"system-resource": SystemResourceSchema,
	"system-identity": SystemIdentitySchema,
	"interface":       InterfaceSchema,
	"ip-address":      IPAddressSchema,
	"ip-route":        IPRouteSchema,
	"dns":             DNSSettingsSchema,
	"firewall-filter": FirewallRuleSchema,
	"dhcp-lease":      DHCPLeaseSchema,
	"dhcp-server":     DHCPServerSchema,
	"dhcp-pool":       DHCPPoolSchema,
}

// Get looks up a schema by command name. It returns the schema and true if
// found, or a zero Schema and false if the name is not registered.
func Get(name string) (Schema, bool) {
	fn, ok := registry[name]
	if !ok {
		return Schema{}, false
	}
	return fn(), true
}

// List returns all registered schema names in sorted order.
func List() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
