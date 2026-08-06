package panel

import (
	"github.com/gameap/gameapctl/pkg/gameap"
)

type Options struct {
	Scope string
}

func (o Options) scope() string {
	return gameap.ScopeOrDefault(o.Scope)
}

func firstOptions(opts []Options) Options {
	if len(opts) > 0 {
		return opts[0]
	}

	return Options{}
}
