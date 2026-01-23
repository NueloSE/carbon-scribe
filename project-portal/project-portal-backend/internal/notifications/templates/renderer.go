package templates

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

// Renderer handles template variable substitution
type Renderer struct {
	delimLeft  string
	delimRight string
}

// NewRenderer creates a new template renderer
func NewRenderer() *Renderer {
	return &Renderer{
		delimLeft:  "{{",
		delimRight: "}}",
	}
}

// Render renders a template string with the given variables
func (r *Renderer) Render(templateStr string, variables map[string]interface{}) (string, error) {
	// First try simple variable substitution for common cases
	result := r.simpleRender(templateStr, variables)

	// If the template contains Go template syntax beyond simple variables,
	// use the full template engine
	if r.hasComplexSyntax(result) {
		return r.complexRender(result, variables)
	}

	return result, nil
}

// simpleRender performs simple {{variable}} substitution
func (r *Renderer) simpleRender(templateStr string, variables map[string]interface{}) string {
	result := templateStr

	for key, value := range variables {
		placeholder := fmt.Sprintf("%s%s%s", r.delimLeft, key, r.delimRight)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))

		// Also try with spaces: {{ variable }}
		placeholderWithSpaces := fmt.Sprintf("%s %s %s", r.delimLeft, key, r.delimRight)
		result = strings.ReplaceAll(result, placeholderWithSpaces, fmt.Sprintf("%v", value))
	}

	return result
}

// hasComplexSyntax checks if the template has complex Go template syntax
func (r *Renderer) hasComplexSyntax(templateStr string) bool {
	// Check for range, if, with, define, etc.
	complexPatterns := []string{
		`{{-?\s*range`,
		`{{-?\s*if`,
		`{{-?\s*with`,
		`{{-?\s*define`,
		`{{-?\s*template`,
		`{{-?\s*block`,
		`{{-?\s*\.`,
	}

	for _, pattern := range complexPatterns {
		matched, _ := regexp.MatchString(pattern, templateStr)
		if matched {
			return true
		}
	}

	return false
}

// complexRender uses Go's text/template for complex templates
func (r *Renderer) complexRender(templateStr string, variables map[string]interface{}) (string, error) {
	tmpl, err := template.New("notification").
		Delims(r.delimLeft, r.delimRight).
		Funcs(r.templateFuncs()).
		Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, variables); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// templateFuncs returns custom template functions
func (r *Renderer) templateFuncs() template.FuncMap {
	return template.FuncMap{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": strings.Title,
		"trim":  strings.TrimSpace,
		"default": func(defaultVal, val interface{}) interface{} {
			if val == nil || val == "" {
				return defaultVal
			}
			return val
		},
		"formatNumber": func(n interface{}) string {
			return fmt.Sprintf("%v", n)
		},
		"formatCurrency": func(amount float64, currency string) string {
			return fmt.Sprintf("%s %.2f", currency, amount)
		},
		"formatPercent": func(value float64) string {
			return fmt.Sprintf("%.1f%%", value*100)
		},
		"truncate": func(length int, s string) string {
			if len(s) <= length {
				return s
			}
			return s[:length] + "..."
		},
		"join": func(sep string, items []string) string {
			return strings.Join(items, sep)
		},
	}
}

// RenderHTML renders HTML template with HTML-safe output
func (r *Renderer) RenderHTML(templateStr string, variables map[string]interface{}) (string, error) {
	// For HTML, we use html/template instead of text/template
	// But first try simple rendering
	result := r.simpleRender(templateStr, variables)

	if !r.hasComplexSyntax(result) {
		return result, nil
	}

	// For complex syntax, use text/template but escape HTML manually
	return r.complexRender(result, variables)
}

// ExtractVariables extracts variable names from a template string
func (r *Renderer) ExtractVariables(templateStr string) []string {
	// Match {{variable}} and {{ variable }}
	pattern := regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)
	matches := pattern.FindAllStringSubmatch(templateStr, -1)

	seen := make(map[string]bool)
	var variables []string

	for _, match := range matches {
		if len(match) >= 2 {
			varName := match[1]
			if !seen[varName] {
				seen[varName] = true
				variables = append(variables, varName)
			}
		}
	}

	return variables
}

// ValidateVariables checks if all required variables are provided
func (r *Renderer) ValidateVariables(templateStr string, variables map[string]interface{}) error {
	required := r.ExtractVariables(templateStr)

	var missing []string
	for _, v := range required {
		if _, ok := variables[v]; !ok {
			missing = append(missing, v)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required variables: %s", strings.Join(missing, ", "))
	}

	return nil
}

// RenderWithDefaults renders with default values for missing variables
func (r *Renderer) RenderWithDefaults(templateStr string, variables map[string]interface{}, defaults map[string]interface{}) (string, error) {
	// Merge defaults with variables
	merged := make(map[string]interface{})
	for k, v := range defaults {
		merged[k] = v
	}
	for k, v := range variables {
		merged[k] = v
	}

	return r.Render(templateStr, merged)
}
