package rosapi

import "sort"

// ---------------------------------------------------------------------------
// SystemResource
// ---------------------------------------------------------------------------

// SystemResource represents the output of /system/resource/print.
type SystemResource struct {
	Uptime           string `ros:"uptime"`
	Version          string `ros:"version"`
	BuildTime        string `ros:"build-time"`
	CPUCount         string `ros:"cpu-count"`
	CPULoad          string `ros:"cpu-load"`
	FreeMemory       string `ros:"free-memory"`
	TotalMemory      string `ros:"total-memory"`
	FreeHDDSpace     string `ros:"free-hdd-space"`
	TotalHDDSpace    string `ros:"total-hdd-space"`
	ArchitectureName string `ros:"architecture-name"`
	BoardName        string `ros:"board-name"`
	Platform         string `ros:"platform"`
}

// SystemResources is a slice of SystemResource that implements Renderable.
type SystemResources []SystemResource

func (s SystemResources) TableHeaders() []string {
	return []string{"Board", "Platform", "Version", "Uptime", "CPU", "CPU Load", "Memory Free/Total", "HDD Free/Total"}
}

func (s SystemResources) TableRows() [][]string {
	rows := make([][]string, len(s))
	for i, r := range s {
		rows[i] = []string{
			r.BoardName,
			r.Platform,
			r.Version,
			r.Uptime,
			r.CPUCount,
			r.CPULoad,
			r.FreeMemory + "/" + r.TotalMemory,
			r.FreeHDDSpace + "/" + r.TotalHDDSpace,
		}
	}
	return rows
}

// ---------------------------------------------------------------------------
// SystemIdentity
// ---------------------------------------------------------------------------

// SystemIdentity represents the output of /system/identity/print.
type SystemIdentity struct {
	Name string `ros:"name"`
}

// SystemIdentities is a slice of SystemIdentity that implements Renderable.
type SystemIdentities []SystemIdentity

func (s SystemIdentities) TableHeaders() []string {
	return []string{"Name"}
}

func (s SystemIdentities) TableRows() [][]string {
	rows := make([][]string, len(s))
	for i, id := range s {
		rows[i] = []string{id.Name}
	}
	return rows
}

// ---------------------------------------------------------------------------
// SystemInfo (combined identity + resource for "system info" command)
// ---------------------------------------------------------------------------

// SystemInfo combines identity and resource data for the system info view.
type SystemInfo struct {
	Identity string
	Resource SystemResource
}

// SystemInfoList is a slice of SystemInfo that implements Renderable.
type SystemInfoList []SystemInfo

func (s SystemInfoList) TableHeaders() []string {
	return []string{"Identity", "Board", "Platform", "Version", "Uptime", "CPU Load", "Memory Free/Total", "HDD Free/Total"}
}

func (s SystemInfoList) TableRows() [][]string {
	rows := make([][]string, len(s))
	for i, info := range s {
		r := info.Resource
		rows[i] = []string{
			info.Identity,
			r.BoardName,
			r.Platform,
			r.Version,
			r.Uptime,
			r.CPULoad + "%",
			r.FreeMemory + "/" + r.TotalMemory,
			r.FreeHDDSpace + "/" + r.TotalHDDSpace,
		}
	}
	return rows
}

// ---------------------------------------------------------------------------
// Interface
// ---------------------------------------------------------------------------

// Interface represents the output of /interface/print.
type Interface struct {
	ID       string `ros:".id"`
	Name     string `ros:"name"`
	Type     string `ros:"type"`
	MTU      string `ros:"actual-mtu"`
	Running  string `ros:"running"`
	Disabled string `ros:"disabled"`
	Comment  string `ros:"comment"`
}

// Interfaces is a slice of Interface that implements Renderable.
type Interfaces []Interface

func (s Interfaces) TableHeaders() []string {
	return []string{"Name", "Type", "MTU", "Running", "Disabled", "Comment"}
}

func (s Interfaces) TableRows() [][]string {
	rows := make([][]string, len(s))
	for i, iface := range s {
		rows[i] = []string{
			iface.Name,
			iface.Type,
			iface.MTU,
			iface.Running,
			iface.Disabled,
			iface.Comment,
		}
	}
	return rows
}

// ---------------------------------------------------------------------------
// IPAddress
// ---------------------------------------------------------------------------

// IPAddress represents the output of /ip/address/print.
type IPAddress struct {
	ID        string `ros:".id"`
	Address   string `ros:"address"`
	Network   string `ros:"network"`
	Interface string `ros:"interface"`
	Disabled  string `ros:"disabled"`
	Comment   string `ros:"comment"`
}

// IPAddresses is a slice of IPAddress that implements Renderable.
type IPAddresses []IPAddress

func (s IPAddresses) TableHeaders() []string {
	return []string{"Address", "Network", "Interface", "Disabled", "Comment"}
}

func (s IPAddresses) TableRows() [][]string {
	rows := make([][]string, len(s))
	for i, a := range s {
		rows[i] = []string{
			a.Address,
			a.Network,
			a.Interface,
			a.Disabled,
			a.Comment,
		}
	}
	return rows
}

// ---------------------------------------------------------------------------
// IPRoute
// ---------------------------------------------------------------------------

// IPRoute represents the output of /ip/route/print.
type IPRoute struct {
	ID         string `ros:".id"`
	DstAddress string `ros:"dst-address"`
	Gateway    string `ros:"gateway"`
	Distance   string `ros:"distance"`
	Routing    string `ros:"routing-table"`
	Disabled   string `ros:"disabled"`
}

// IPRoutes is a slice of IPRoute that implements Renderable.
type IPRoutes []IPRoute

func (s IPRoutes) TableHeaders() []string {
	return []string{"Dst Address", "Gateway", "Distance", "Routing Table", "Disabled"}
}

func (s IPRoutes) TableRows() [][]string {
	rows := make([][]string, len(s))
	for i, r := range s {
		rows[i] = []string{
			r.DstAddress,
			r.Gateway,
			r.Distance,
			r.Routing,
			r.Disabled,
		}
	}
	return rows
}

// ---------------------------------------------------------------------------
// FirewallRule
// ---------------------------------------------------------------------------

// FirewallRule represents the output of /ip/firewall/filter/print (or nat/mangle).
type FirewallRule struct {
	ID         string `ros:".id"`
	Chain      string `ros:"chain"`
	Action     string `ros:"action"`
	Protocol   string `ros:"protocol"`
	SrcAddress string `ros:"src-address"`
	DstAddress string `ros:"dst-address"`
	DstPort    string `ros:"dst-port"`
	Comment    string `ros:"comment"`
	Disabled   string `ros:"disabled"`
}

// FirewallRules is a slice of FirewallRule that implements Renderable.
type FirewallRules []FirewallRule

func (s FirewallRules) TableHeaders() []string {
	return []string{"Chain", "Action", "Protocol", "Src Address", "Dst Address", "Dst Port", "Comment", "Disabled"}
}

func (s FirewallRules) TableRows() [][]string {
	rows := make([][]string, len(s))
	for i, r := range s {
		rows[i] = []string{
			r.Chain,
			r.Action,
			r.Protocol,
			r.SrcAddress,
			r.DstAddress,
			r.DstPort,
			r.Comment,
			r.Disabled,
		}
	}
	return rows
}

// ---------------------------------------------------------------------------
// DHCPLease
// ---------------------------------------------------------------------------

// DHCPLease represents the output of /ip/dhcp-server/lease/print.
type DHCPLease struct {
	ID         string `ros:".id"`
	Address    string `ros:"address"`
	MACAddress string `ros:"mac-address"`
	HostName   string `ros:"host-name"`
	Status     string `ros:"status"`
	Comment    string `ros:"comment"`
}

// DHCPLeases is a slice of DHCPLease that implements Renderable.
type DHCPLeases []DHCPLease

func (s DHCPLeases) TableHeaders() []string {
	return []string{"Address", "MAC Address", "Host Name", "Status", "Comment"}
}

func (s DHCPLeases) TableRows() [][]string {
	rows := make([][]string, len(s))
	for i, l := range s {
		rows[i] = []string{
			l.Address,
			l.MACAddress,
			l.HostName,
			l.Status,
			l.Comment,
		}
	}
	return rows
}

// ---------------------------------------------------------------------------
// DNSSettings
// ---------------------------------------------------------------------------

// DNSSettings represents the output of /ip/dns/print.
type DNSSettings struct {
	Servers              string `ros:"servers"`
	DynamicServers       string `ros:"dynamic-servers"`
	AllowRemoteRequests  string `ros:"allow-remote-requests"`
	MaxUDPPacketSize     string `ros:"max-udp-packet-size"`
	CacheSize            string `ros:"cache-size"`
	CacheMaxTTL          string `ros:"cache-max-ttl"`
	CacheUsed            string `ros:"cache-used"`
}

type DNSSettingsList []DNSSettings

func (s DNSSettingsList) TableHeaders() []string {
	return []string{"Servers", "Dynamic Servers", "Allow Remote", "Cache Size", "Cache Used"}
}

func (s DNSSettingsList) TableRows() [][]string {
	rows := make([][]string, len(s))
	for i, d := range s {
		rows[i] = []string{d.Servers, d.DynamicServers, d.AllowRemoteRequests, d.CacheSize, d.CacheUsed}
	}
	return rows
}

// ---------------------------------------------------------------------------
// DHCPServer
// ---------------------------------------------------------------------------

// DHCPServer represents the output of /ip/dhcp-server/print.
type DHCPServer struct {
	ID        string `ros:".id"`
	Name      string `ros:"name"`
	Interface string `ros:"interface"`
	Disabled  string `ros:"disabled"`
	AddressPool string `ros:"address-pool"`
	LeaseTime string `ros:"lease-time"`
}

type DHCPServers []DHCPServer

func (s DHCPServers) TableHeaders() []string {
	return []string{"Name", "Interface", "Address Pool", "Lease Time", "Disabled"}
}

func (s DHCPServers) TableRows() [][]string {
	rows := make([][]string, len(s))
	for i, d := range s {
		rows[i] = []string{d.Name, d.Interface, d.AddressPool, d.LeaseTime, d.Disabled}
	}
	return rows
}

// ---------------------------------------------------------------------------
// DHCPPool
// ---------------------------------------------------------------------------

// DHCPPool represents the output of /ip/pool/print.
type DHCPPool struct {
	ID      string `ros:".id"`
	Name    string `ros:"name"`
	Ranges  string `ros:"ranges"`
	Comment string `ros:"comment"`
}

type DHCPPools []DHCPPool

func (s DHCPPools) TableHeaders() []string {
	return []string{"Name", "Ranges", "Comment"}
}

func (s DHCPPools) TableRows() [][]string {
	rows := make([][]string, len(s))
	for i, p := range s {
		rows[i] = []string{p.Name, p.Ranges, p.Comment}
	}
	return rows
}

// ---------------------------------------------------------------------------
// TrafficStats (for monitor traffic)
// ---------------------------------------------------------------------------

// TrafficStats represents interface traffic counters.
type TrafficStats struct {
	Name     string `ros:"name"`
	RxByte   string `ros:"rx-byte"`
	TxByte   string `ros:"tx-byte"`
	RxPacket string `ros:"rx-packet"`
	TxPacket string `ros:"tx-packet"`
}

type TrafficStatsList []TrafficStats

func (s TrafficStatsList) TableHeaders() []string {
	return []string{"Name", "RX Bytes", "TX Bytes", "RX Packets", "TX Packets"}
}

func (s TrafficStatsList) TableRows() [][]string {
	rows := make([][]string, len(s))
	for i, t := range s {
		rows[i] = []string{t.Name, t.RxByte, t.TxByte, t.RxPacket, t.TxPacket}
	}
	return rows
}

// ---------------------------------------------------------------------------
// GenericResult (for the exec command)
// ---------------------------------------------------------------------------

// GenericResult holds an arbitrary RouterOS sentence as a raw key-value map.
// It is used by the exec command where the response structure is not known
// ahead of time.
type GenericResult struct {
	Fields map[string]string
}

// GenericResults is a slice of GenericResult that implements Renderable.
// Headers are auto-detected by collecting all unique keys across every result,
// sorted alphabetically for stable output. Use SetKeyOrder to override.
type GenericResults struct {
	Items    []GenericResult
	keyOrder []string
}

// SetKeyOrder overrides the auto-detected header order.
func (g *GenericResults) SetKeyOrder(keys []string) {
	g.keyOrder = keys
}

func (g GenericResults) TableHeaders() []string {
	if len(g.keyOrder) > 0 {
		return g.keyOrder
	}
	seen := make(map[string]struct{})
	for _, r := range g.Items {
		for k := range r.Fields {
			seen[k] = struct{}{}
		}
	}

	headers := make([]string, 0, len(seen))
	for k := range seen {
		headers = append(headers, k)
	}
	sort.Strings(headers)
	return headers
}

func (g GenericResults) TableRows() [][]string {
	headers := g.TableHeaders()
	rows := make([][]string, len(g.Items))
	for i, r := range g.Items {
		row := make([]string, len(headers))
		for j, h := range headers {
			row[j] = r.Fields[h]
		}
		rows[i] = row
	}
	return rows
}
