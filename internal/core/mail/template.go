package mail

import (
	"bytes"
	"fmt"
	"html/template"
)

// Renderer renders HTML email bodies from named templates with automatic
// HTML escaping (html/template).
type Renderer struct {
	tmpl *template.Template
}

// NewRenderer parses the given named templates (name -> template source).
func NewRenderer(templates map[string]string) (*Renderer, error) {
	root := template.New("skolva-mail")
	for name, body := range templates {
		if _, err := root.New(name).Parse(body); err != nil {
			return nil, fmt.Errorf("parse template %q: %w", name, err)
		}
	}
	return &Renderer{tmpl: root}, nil
}

// Render executes the named template with data and returns the rendered HTML.
func (r *Renderer) Render(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := r.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("render template %q: %w", name, err)
	}
	return buf.String(), nil
}
