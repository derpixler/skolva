package mail

import (
	"strings"
	"testing"
)

func TestRendererRender(t *testing.T) {
	r, err := NewRenderer(map[string]string{
		"otp": "<p>Code: {{.Code}}</p>",
	})
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	out, err := r.Render("otp", map[string]string{"Code": "123456"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if out != "<p>Code: 123456</p>" {
		t.Errorf("unexpected render output: %q", out)
	}
}

func TestRendererAutoEscapes(t *testing.T) {
	r, err := NewRenderer(map[string]string{"x": "<b>{{.Name}}</b>"})
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	out, err := r.Render("x", map[string]string{"Name": "<script>alert(1)</script>"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(out, "<script>") {
		t.Errorf("expected HTML to be escaped, got %q", out)
	}
}

func TestRendererParseError(t *testing.T) {
	if _, err := NewRenderer(map[string]string{"bad": "{{.Unclosed"}); err == nil {
		t.Error("expected parse error for malformed template")
	}
}

func TestRendererUnknownTemplate(t *testing.T) {
	r, err := NewRenderer(map[string]string{"a": "x"})
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	if _, err := r.Render("missing", nil); err == nil {
		t.Error("expected error for unknown template")
	}
}
