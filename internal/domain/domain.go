package domain

type Scope [2]string

func NewScope(ns, db string) Scope {
	return Scope{ns, db}
}

func (s Scope) String() string {
	return s[0] + ":" + s[1]
}

func (s Scope) Namespace() string {
	return s[0]
}

func (s Scope) Database() string {
	return s[1]
}

type SurrealDBInfo struct {
	Accesses   map[string]interface{} `json:"accesses"`
	Namespaces map[string]interface{} `json:"namespaces"`

	Nodes  map[string]interface{} `json:"nodes"`
	System SurrealDBSystemInfo    `json:"system"`
	Users  map[string]interface{} `json:"users"`
}

type SurrealDBSystemInfo struct {
	AvailableParallelism int       `json:"available_parallelism"`
	CpuUsage             float64   `json:"cpu_usage"`
	LoadAverage          []float64 `json:"load_average"`
	MemoryAllocated      int       `json:"memory_allocated"`
	MemoryUsage          int       `json:"memory_usage"`
	PhysicalCores        int       `json:"physical_cores"`
	Threads              int       `json:"threads"`
}

func (s *SurrealDBInfo) AvailableParallelism() float64 {
	return float64(s.System.AvailableParallelism)
}

func (s *SurrealDBInfo) ListNamespaces() []string {
	l := make([]string, 0, len(s.Namespaces))

	for k := range s.Namespaces {
		l = append(l, k)
	}

	return l
}
