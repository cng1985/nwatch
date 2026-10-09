package checker

import "fmt"

type Registry struct {
	items map[string]Checker
}

func NewRegistry(http *HTTPChecker, tlsChecker *TLSChecker, tcp *TCPChecker, cpu *CPUChecker, memory *MemoryChecker, disk *DiskChecker, script *ScriptChecker) *Registry {
	r := &Registry{items: map[string]Checker{}}
	for _, c := range []Checker{http, tlsChecker, tcp, cpu, memory, disk, script} {
		r.items[c.Type()] = c
	}
	return r
}

func (r *Registry) Get(kind string) (Checker, bool) {
	c, ok := r.items[kind]
	return c, ok
}

func (r *Registry) Must(kind string) (Checker, error) {
	c, ok := r.Get(kind)
	if !ok {
		return nil, fmt.Errorf("未注册的监控类型 %s", kind)
	}
	return c, nil
}
