// internal/template/template.go
package template

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"text/template/parse"

	"github.com/gxo-labs/gxo/internal/secrets"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/events"
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	pkgsecrets "github.com/gxo-labs/gxo/pkg/gxo/v1/secrets"
)

const (
	GxoStateKeyPrefix = "_gxo"
)

var simpleVarRegex = regexp.MustCompile(`^\s*\{\{\s*\.([a-zA-Z0-9_.]+)\s*\}\}\s*$`)

// Renderer defines the interface for GXO's templating engine.
type Renderer interface {
	Render(templateString string, data interface{}) (string, error)
	Resolve(templateString string, data interface{}) (interface{}, error)
	ExtractVariables(templateString string) ([]string, error)
	GetFuncMap() template.FuncMap
}

// GoRenderer implements the Renderer interface using Go's text/template package.
// It includes caching for parsed templates and extracted variables to improve performance.
// This struct is designed to be concurrency-safe.
type GoRenderer struct {
	secretsProvider pkgsecrets.Provider
	eventBus        events.Bus
	secretTracker   *secrets.SecretTracker // Holds the per-task tracker
	templateCache   map[string]*template.Template
	varCache        map[string][]string
	mu              sync.Mutex // A single mutex to protect both caches and the non-thread-safe Parse call.
}

// NewGoRenderer creates a new GoRenderer instance. It should be called per task execution
// to ensure the correct secretTracker is bound for redaction.
func NewGoRenderer(secretsProvider pkgsecrets.Provider, eventBus events.Bus, tracker *secrets.SecretTracker) *GoRenderer {
	return &GoRenderer{
		secretsProvider: secretsProvider,
		eventBus:        eventBus,
		secretTracker:   tracker, // Initialize with the passed tracker
		templateCache:   make(map[string]*template.Template),
		varCache:        make(map[string][]string),
	}
}

// GetFuncMap creates and returns the standard function map for GXO templates.
// It uses the GoRenderer's internal secretsProvider, eventBus, and secretTracker.
func (r *GoRenderer) GetFuncMap() template.FuncMap {
	return GetFuncMap(r.secretsProvider, r.eventBus, r.secretTracker)
}

// Render executes a template against the given data using the renderer's FuncMap.
func (r *GoRenderer) Render(templateString string, data interface{}) (string, error) {
	t, err := r.getOrParseTemplate(templateString, r.GetFuncMap())
	if err != nil {
		return "", gxoerrors.NewValidationError(fmt.Sprintf("template parse error: %s", err.Error()), err)
	}

	var buf bytes.Buffer
	if execErr := t.Execute(&buf, data); execErr != nil {
		return "", gxoerrors.NewValidationError(fmt.Sprintf("template execution error: %s", execErr.Error()), execErr)
	}

	return buf.String(), nil
}

// Resolve attempts to directly resolve a template variable if it's a simple expression,
// falling back to full rendering if not.
func (r *GoRenderer) Resolve(templateString string, data interface{}) (interface{}, error) {
	matches := simpleVarRegex.FindStringSubmatch(templateString)
	if len(matches) == 2 {
		path := matches[1]
		if mapData, ok := data.(map[string]interface{}); ok {
			value, found := lookup(mapData, path)
			if found {
				return value, nil
			}
		}
	}

	// Fallback to full rendering for complex expressions or non-map data.
	return r.Render(templateString, data)
}

// ExtractVariables parses a template string to find all referenced variables.
func (r *GoRenderer) ExtractVariables(templateString string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cachedVars, exists := r.varCache[templateString]; exists {
		return cachedVars, nil
	}

	// Use the renderer's own FuncMap for parsing, ensuring any custom functions
	// are known during AST traversal.
	parseFuncMap := r.GetFuncMap()
	t, parseErr := template.New("extract").Option("missingkey=error").Funcs(parseFuncMap).Parse(templateString)
	if parseErr != nil {
		// Return nil, nil if the template string itself is unparsable.
		// The logical validation phase will catch template parsing errors.
		return nil, nil
	}

	variablesMap := make(map[string]struct{})
	if t.Root != nil {
		extractNodeVariablesRecursive(t.Root, variablesMap, parseFuncMap)
	}

	variables := make([]string, 0, len(variablesMap))
	for v := range variablesMap {
		variables = append(variables, v)
	}

	r.varCache[templateString] = variables
	return variables, nil
}

// getOrParseTemplate is a concurrency-safe method for parsing and caching templates.
func (r *GoRenderer) getOrParseTemplate(templateString string, funcMap template.FuncMap) (*template.Template, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cachedTemplate, exists := r.templateCache[templateString]; exists {
		// Clone and re-apply Funcs: Crucial to ensure the template is always
		// using the *current* FuncMap (and thus the correct captured tracker instance).
		clonedTemplate, err := cachedTemplate.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone cached template: %w", err)
		}
		return clonedTemplate.Funcs(funcMap), nil
	}

	t, parseErr := template.New(templateString).Option("missingkey=error").Funcs(funcMap).Parse(templateString)
	if parseErr != nil {
		return nil, fmt.Errorf("template parse error: %w", parseErr)
	}

	r.templateCache[templateString] = t
	return t, nil
}

func lookup(data map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	var current interface{} = data
	for _, part := range parts {
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = currentMap[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func getFullVarPath(node parse.Node, funcMap template.FuncMap) string {
	switch n := node.(type) {
	case *parse.FieldNode:
		if len(n.Ident) > 0 {
			firstIdent := n.Ident[0]
			if _, isFunc := funcMap[firstIdent]; !isFunc {
				return strings.Join(n.Ident, ".")
			}
		}
	case *parse.ChainNode:
		if fieldNode, isField := n.Node.(*parse.FieldNode); isField {
			return getFullVarPath(fieldNode, funcMap)
		}
		if idNode, isId := n.Node.(*parse.IdentifierNode); isId {
			return getFullVarPath(idNode, funcMap)
		}
	case *parse.VariableNode:
		return ""
	case *parse.DotNode:
		return ""
	case *parse.IdentifierNode:
		if _, isFunc := funcMap[n.Ident]; !isFunc {
			return n.Ident
		}
	}
	return ""
}

func extractNodeVariablesRecursive(node parse.Node, vars map[string]struct{}, funcMap template.FuncMap) {
	if node == nil {
		return
	}

	fullPath := getFullVarPath(node, funcMap)
	if fullPath != "" {
		vars[fullPath] = struct{}{}
	}

	switch n := node.(type) {
	case *parse.ListNode:
		if n != nil {
			for _, subNode := range n.Nodes {
				extractNodeVariablesRecursive(subNode, vars, funcMap)
			}
		}
	case *parse.ActionNode:
		if n.Pipe != nil {
			extractNodeVariablesRecursive(n.Pipe, vars, funcMap)
		}
	case *parse.IfNode:
		extractNodeVariablesRecursive(n.Pipe, vars, funcMap)
		extractNodeVariablesRecursive(n.List, vars, funcMap)
		extractNodeVariablesRecursive(n.ElseList, vars, funcMap)
	case *parse.RangeNode:
		extractNodeVariablesRecursive(n.Pipe, vars, funcMap)
		extractNodeVariablesRecursive(n.List, vars, funcMap)
		extractNodeVariablesRecursive(n.ElseList, vars, funcMap)
	case *parse.WithNode:
		extractNodeVariablesRecursive(n.Pipe, vars, funcMap)
		extractNodeVariablesRecursive(n.List, vars, funcMap)
		extractNodeVariablesRecursive(n.ElseList, vars, funcMap)
	case *parse.PipeNode:
		for _, cmd := range n.Cmds {
			for _, arg := range cmd.Args {
				extractNodeVariablesRecursive(arg, vars, funcMap)
			}
		}
	case *parse.TemplateNode:
	}
}