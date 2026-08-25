package daemon

import "github.com/gameap/gameapctl/pkg/gameap"

type Options struct {
	Scope    string
	WorkPath string
}

func (o Options) scope() string {
	return gameap.ScopeOrDefault(o.Scope)
}

// workPath returns the configured work path or the platform default.
func (o Options) workPath() string {
	if o.WorkPath == "" {
		return gameap.DefaultWorkPath
	}

	return o.WorkPath
}

func firstOptions(opts []Options) Options {
	if len(opts) > 0 {
		return opts[0]
	}

	return Options{}
}
