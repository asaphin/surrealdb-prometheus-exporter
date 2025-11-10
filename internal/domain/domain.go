package domain

type SurrealDBInfo struct {
	Root       SurrealDBRootInfo        `json:"root"`
	Namespaces []SurrealDBNamespaceInfo `json:"namespaces"`
	Databases  []SurrealDBDatabaseInfo  `json:"databases"`
	Tables     []SurrealDBTableInfo     `json:"tables"`
	Indexes    []SurrealDBIndexInfo     `json:"indexes"`
}

type SurrealDBRootInfo struct {
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

type SurrealDBNamespaceInfo struct {
	Accesses  map[string]interface{} `json:"accesses"`
	Databases map[string]interface{} `json:"databases"`
	Users     map[string]interface{} `json:"users"`
}

type SurrealDBDatabaseInfo struct {
	Accesses  map[string]interface{} `json:"accesses"`
	Analyzers map[string]interface{} `json:"analyzers"`
	Apis      map[string]interface{} `json:"apis"`
	Configs   map[string]interface{} `json:"configs"`
	Functions map[string]interface{} `json:"functions"`
	Models    map[string]interface{} `json:"models"`
	Params    map[string]interface{} `json:"params"`
	Tables    map[string]interface{} `json:"tables"`
	Users     map[string]interface{} `json:"users"`
}

type SurrealDBTableInfo struct {
	Events  map[string]interface{} `json:"events"`
	Fields  map[string]interface{} `json:"fields"`
	Indexes map[string]interface{} `json:"indexes"`
	Lives   map[string]interface{} `json:"lives"`
	Tables  map[string]interface{} `json:"tables"`
}

type SurrealDBIndexInfo struct {
	Building struct {
		Initial int    `json:"initial"`
		Pending int    `json:"pending"`
		Status  string `json:"status"`
		Updated int    `json:"updated"`
	} `json:"building"`
}

func (s *SurrealDBRootInfo) AvailableParallelism() float64 {
	return float64(s.System.AvailableParallelism)
}

func (s *SurrealDBRootInfo) ListNamespaces() []string {
	l := make([]string, 0, len(s.Namespaces))

	for k := range s.Namespaces {
		l = append(l, k)
	}

	return l
}
