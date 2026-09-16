package main

const (
	// sortDirectionAsc and sortDirectionDesc are the direction values every search tool accepts.
	sortDirectionAsc  = "asc"
	sortDirectionDesc = "desc"
)

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
