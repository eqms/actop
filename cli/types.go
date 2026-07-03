package cli

import "time"

// ListItem is one element of `container ls -a --format json` and
// `container inspect <id>` (which returns a single-element array).
type ListItem struct {
	ID            string        `json:"id"`
	Configuration Configuration `json:"configuration"`
	// Status is present for existing containers; state is e.g. "running" or "stopped".
	Status *Status `json:"status,omitempty"`
}

type Configuration struct {
	ID           string            `json:"id"`
	CreationDate time.Time         `json:"creationDate"`
	Image        ImageRef          `json:"image"`
	InitProcess  InitProcess       `json:"initProcess"`
	Labels       map[string]string `json:"labels"`
	Resources    Resources         `json:"resources"`
	Platform     Platform          `json:"platform"`
}

type ImageRef struct {
	Reference string `json:"reference"`
}

type InitProcess struct {
	Executable       string   `json:"executable"`
	Arguments        []string `json:"arguments"`
	Environment      []string `json:"environment"`
	WorkingDirectory string   `json:"workingDirectory"`
}

type Resources struct {
	CPUs          int   `json:"cpus"`
	MemoryInBytes int64 `json:"memoryInBytes"`
}

type Platform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
}

type Status struct {
	State       string              `json:"state"`
	StartedDate time.Time           `json:"startedDate"`
	Networks    []NetworkAttachment `json:"networks"`
}

type NetworkAttachment struct {
	Hostname    string `json:"hostname"`
	IPv4Address string `json:"ipv4Address"`
	IPv4Gateway string `json:"ipv4Gateway"`
	MacAddress  string `json:"macAddress"`
	Network     string `json:"network"`
}

// StatsEntry is one element of `container stats --no-stream --format json`.
// CPU, network and block values are cumulative counters since container start.
type StatsEntry struct {
	ID               string `json:"id"`
	CPUUsageUsec     int64  `json:"cpuUsageUsec"`
	MemoryUsageBytes int64  `json:"memoryUsageBytes"`
	MemoryLimitBytes int64  `json:"memoryLimitBytes"`
	NetworkRxBytes   int64  `json:"networkRxBytes"`
	NetworkTxBytes   int64  `json:"networkTxBytes"`
	BlockReadBytes   int64  `json:"blockReadBytes"`
	BlockWriteBytes  int64  `json:"blockWriteBytes"`
	NumProcesses     int    `json:"numProcesses"`
}

// SystemStatus is `container system status --format json`.
type SystemStatus struct {
	Status           string `json:"status"`
	APIServerVersion string `json:"apiServerVersion"`
	AppRoot          string `json:"appRoot"`
}

// IP returns the primary IPv4 address (without CIDR suffix), or "".
func (s *Status) IP() string {
	if s == nil || len(s.Networks) == 0 {
		return ""
	}
	addr := s.Networks[0].IPv4Address
	for i := range addr {
		if addr[i] == '/' {
			return addr[:i]
		}
	}
	return addr
}
