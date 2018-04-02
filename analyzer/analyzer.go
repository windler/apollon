package analyzer

import (
	"os"
	"sort"

	"github.com/cespare/xxhash"

	cachegrind "github.com/windler/go-cachegrind"
)

const (
	Time       = "Time"
	Memory     = "Memory"
	UnitTime   = "sec"
	UnitMemory = "kB"
)

type CachegrindParser struct {
	cg          cachegrind.Cachegrind
	calls       []*Call
	memoryCalls []*Call
	timeCalls   []*Call
	main        *Call
}

type Call struct {
	Time, Memory                               float32
	CallerFn, CallerFile, CalleeFn, CalleeFile string
	Line, Occurences                           int
}

func NewFrom(file string) (*CachegrindParser, error) {
	p := &CachegrindParser{}

	if _, err := os.Stat(file); err != nil {
		return nil, err
	}

	cg, err := cachegrind.Parse(file)
	if err != nil {
		return nil, err
	}

	p.cg = cg

	p.initialize()
	p.prepareMemory()
	p.prepareTime()

	return p, nil
}

func (p *CachegrindParser) initialize() {
	main := p.cg.GetMainFunction()
	if main == nil {
		panic("There is no main function in your cachgrind file.")
	}
	calls := getCalls(main)

	p.main = calls[0]
	p.calls = calls
}

func (p *CachegrindParser) prepareMemory() {
	p.memoryCalls = p.createSortedFunctionCalls(Memory)
}

func (p *CachegrindParser) prepareTime() {
	p.timeCalls = p.createSortedFunctionCalls(Time)
}

func (p *CachegrindParser) createSortedFunctionCalls(kind string) []*Call {
	res := []*Call{}
	switch kind {
	case Time:
		sort.SliceStable(p.calls, func(i, j int) bool {
			return p.calls[i].Time > p.calls[j].Time
		})
	case Memory:
		sort.SliceStable(p.calls, func(i, j int) bool {
			return p.calls[i].Memory > p.calls[j].Memory
		})
	}

	for i := 0; i < len(p.calls); i++ {
		res = append(res, p.calls[i])
	}

	return res
}

func (p *CachegrindParser) GetFirst() *Call {
	return p.main
}

func (p *CachegrindParser) GetTop(n int, kind string) []*Call {
	switch kind {
	case Time:
		if len(p.timeCalls) >= n {
			return p.timeCalls[:n]
		}
		return p.timeCalls
	case Memory:
		if len(p.memoryCalls) >= n {
			return p.memoryCalls[:n]
		}
		return p.memoryCalls
	}

	panic("Kind not valid.")
}

func (p *CachegrindParser) GetMainMeasurements(kind string) float32 {
	switch kind {
	case Time:
		return float32(p.cg.GetMainFunction().GetMeasurement(kind)) / 1000 / 1000
	case Memory:
		return float32(p.cg.GetMainFunction().GetMeasurement(Memory)) / 1000
	}
	panic("Kind not valid.")
}

var callCache = map[uint64]*Call{}

func getCalls(fn cachegrind.Function) []*Call {
	calls := []*Call{}
	for _, call := range fn.GetCalls() {
		newCall := &Call{
			CallerFn:   fn.GetName(),
			CallerFile: fn.GetFile(),
			CalleeFn:   call.GetFunction().GetName(),
			CalleeFile: call.GetFunction().GetFile(),
			Line:       call.GetLine(),
			Time:       float32(call.GetMeasurement(Time)) / 1000 / 1000,
			Memory:     float32(call.GetMeasurement(Memory)) / 1000,
			Occurences: 1,
		}

		id := xxhash.Sum64String(newCall.CalleeFn + newCall.CalleeFile + newCall.CallerFn + newCall.CallerFile)
		if callCache[id] == nil {
			callCache[id] = newCall
			calls = append(calls, newCall)
			calls = append(calls, getCalls(call.GetFunction())...)
		} else {
			callCache[id].Occurences = callCache[id].Occurences + 1
		}
	}

	return calls

}
