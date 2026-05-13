package daemon

import "github.com/gameap/gameapctl/pkg/gameap"

type Options struct {
	Scope string
}

func (o Options) scope() string {
	if o.Scope == "" {
		return gameap.ScopeSystem
	}

	return o.Scope
}

func firstOptions(opts []Options) Options {
	if len(opts) > 0 {
		return opts[0]
	}

	return Options{}
}
