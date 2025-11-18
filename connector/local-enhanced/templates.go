package local

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"
)

//go:embed templates/*.html
var templatesFS embed.FS

// Templates holds all parsed HTML templates for the connector.
type Templates struct {
	login            *template.Template
	setupAuth        *template.Template
	manageCredentials *template.Template
	twofaPrompt      *template.Template
}

// LoadTemplates loads and parses all HTML templates from the embedded filesystem.
func LoadTemplates() (*Templates, error) {
	// Parse all templates with base functions
	funcs := template.FuncMap{
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
		"upper": func(s string) string {
			return strings.ToUpper(s)
		},
		"formatDate": func(t time.Time) string {
			return t.Format("Jan 2, 2006 3:04 PM")
		},
		"contains": func(slice []string, item string) bool {
			for _, s := range slice {
				if s == item {
					return true
				}
			}
			return false
		},
	}

	// Parse all template files
	tmpls, err := template.New("").Funcs(funcs).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	// Verify all required templates exist
	requiredTemplates := []string{
		"login.html",
		"setup-auth.html",
		"manage-credentials.html",
		"twofa-prompt.html",
	}

	for _, tmplName := range requiredTemplates {
		if tmpls.Lookup(tmplName) == nil {
			return nil, fmt.Errorf("missing required template: %s", tmplName)
		}
	}

	return &Templates{
		login:             tmpls.Lookup("login.html"),
		setupAuth:         tmpls.Lookup("setup-auth.html"),
		manageCredentials: tmpls.Lookup("manage-credentials.html"),
		twofaPrompt:       tmpls.Lookup("twofa-prompt.html"),
	}, nil
}

// RenderLogin renders the login page template.
func (t *Templates) RenderLogin(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.login.Execute(w, data); err != nil {
		return fmt.Errorf("failed to render login template: %w", err)
	}
	return nil
}

// RenderSetupAuth renders the authentication setup page template.
func (t *Templates) RenderSetupAuth(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.setupAuth.Execute(w, data); err != nil {
		return fmt.Errorf("failed to render setup-auth template: %w", err)
	}
	return nil
}

// RenderManageCredentials renders the credential management page template.
func (t *Templates) RenderManageCredentials(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.manageCredentials.Execute(w, data); err != nil {
		return fmt.Errorf("failed to render manage-credentials template: %w", err)
	}
	return nil
}

// Render2FAPrompt renders the 2FA prompt page template.
func (t *Templates) Render2FAPrompt(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.twofaPrompt.Execute(w, data); err != nil {
		return fmt.Errorf("failed to render twofa-prompt template: %w", err)
	}
	return nil
}
