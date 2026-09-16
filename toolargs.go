package main

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

// intArg returns the integer argument stored under key, or fallback when the
// argument is absent or not a number. JSON numbers decode into float64.
func intArg(args map[string]any, key string, fallback int) int {
	raw, ok := args[key]
	if !ok {
		return fallback
	}
	value, isFloat := raw.(float64)
	if !isFloat {
		return fallback
	}
	return int(value)
}
