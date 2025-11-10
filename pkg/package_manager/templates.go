package packagemanager

import (
	"text/template"

	pkgstings "github.com/gameap/gameapctl/pkg/strings"
)

var runtimeTemplateFuncMap = template.FuncMap{
	"default": func(defaultVal interface{}, value interface{}) interface{} {
		if value == nil || value == "" {
			return defaultVal
		}

		return value
	},
	"generatePassword": pkgstings.GeneratePassword,
}
