package main

import "fmt"

const (
	// sortDirectionAsc and sortDirectionDesc are the direction values every search tool accepts.
	sortDirectionAsc  = "asc"
	sortDirectionDesc = "desc"
	// searchSortDefault is the sort key both deck-search tools default to, so the two
	// tools share names, defaults and semantics across their common parameters.
	searchSortDefault = "views"
)

// sortDirectionLabel renders a sort direction in the vocabulary the caller passed.
// Upstream wire values (Moxfield's "Descending", Archidekt's "-" prefix) must never
// reach the output, or a tool would echo a direction the caller cannot type back.
func sortDirectionLabel(ascending bool) string {
	if ascending {
		return sortDirectionAsc
	}
	return sortDirectionDesc
}

// stringArg returns the string argument stored under key, or fallback when the
// argument is absent, not a string, or empty. MCP delivers arguments as untyped JSON.
func stringArg(args map[string]any, key, fallback string) string {
	raw, ok := args[key]
	if !ok {
		return fallback
	}
	value, isString := raw.(string)
	if !isString || value == "" {
		return fallback
	}
	return value
}

// enumArg returns the enum argument stored under key, or fallback when the argument is
// absent. A present argument must be a non-empty string: an empty or wrong-typed enum is
// a caller mistake, and substituting the default would silently answer a different
// question than the one asked.
func enumArg(args map[string]any, key, fallback string) (string, error) {
	raw, ok := args[key]
	if !ok {
		return fallback, nil
	}
	value, isString := raw.(string)
	if !isString {
		return "", fmt.Errorf("invalid %s: expected a string, got %T", key, raw)
	}
	if value == "" {
		return "", fmt.Errorf("invalid %s: expected a non-empty value", key)
	}
	return value, nil
}

// intArg returns the integer argument stored under key, or fallback when the argument is
// absent. JSON numbers decode into float64; any other type is a caller mistake and is
// rejected rather than replaced by the default.
func intArg(args map[string]any, key string, fallback int) (int, error) {
	raw, ok := args[key]
	if !ok {
		return fallback, nil
	}
	value, isFloat := raw.(float64)
	if !isFloat {
		return 0, fmt.Errorf("invalid %s: expected a number, got %T", key, raw)
	}
	return int(value), nil
}
