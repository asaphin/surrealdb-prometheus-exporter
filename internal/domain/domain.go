package domain

type SurrealDBInfo struct {
	Accesses   map[string]interface{} `json:"accesses"`
	Namespaces map[string]interface{} `json:"namespaces"`
	Nodes      map[string]interface{} `json:"nodes"`
	System     SurrealDBSystemInfo    `json:"system"`
	Users      map[string]interface{} `json:"users"`
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
