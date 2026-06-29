package handlers

import "html/template"

// templateFuncMap is shared across all handler-rendered templates so pointer
// fields (used throughout models for nullable DB columns) can be displayed
// safely — printing a *string directly via fmt prints its address, not its
// value, so every template that touches one must go through deref.
var templateFuncMap = template.FuncMap{
	"deref": func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	},
	"isNil": func(s *string) bool { return s == nil },
	"mul":   func(a, b int) int { return a * b },
}

func parseTemplates(files ...string) (*template.Template, error) {
	return template.New("root").Funcs(templateFuncMap).ParseFiles(files...)
}
