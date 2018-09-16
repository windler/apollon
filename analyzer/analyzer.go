package analyzer

import cachegrind "github.com/windler/go-cachegrind"

const (
	Time       = "Time"
	Memory     = "Memory"
	UnitTime   = "sec"
	UnitMemory = "kB"
)

type CachegrindAnalyzer interface {
	Init(c cachegrind.Cachegrind)
	GetTopNPrefixCalls(n int, prefix string) interface{}
}
