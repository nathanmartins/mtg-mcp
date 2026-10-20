package mtgserver

import (
	"errors"
	"fmt"
	"math"
)

// errInvalid prefixes every rejected tool argument ("invalid limit 0 (...)"), so the
// caller-facing wording stays uniform across tools.
var errInvalid = errors.New("invalid")

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

// sortAscendingArg reads the sort_direction argument (asc or desc, default desc) and reports
// whether it asks for ascending order. Each deck-search client maps the result onto its own
// wire format, so the tool vocabulary stays in the handlers.
func sortAscendingArg(args map[string]any) (bool, error) {
	direction, err := enumArg(args, "sort_direction", sortDirectionDesc)
	if err != nil {
		return false, err
	}
	switch direction {
	case sortDirectionAsc:
		return true, nil
	case sortDirectionDesc:
		return false, nil
	default:
		return false, fmt.Errorf("%w sort_direction %q (accepted: %s, %s)",
			errInvalid, direction, sortDirectionAsc, sortDirectionDesc)
	}
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
		return "", fmt.Errorf("%w %s: expected a string, got %T", errInvalid, key, raw)
	}
	if value == "" {
		return "", fmt.Errorf("%w %s: expected a non-empty value", errInvalid, key)
	}
	return value, nil
}

// cappedIntArg reads an optional integer argument the way intArg does, then rejects values
// below minimum and caps values above maximum, matching how the tools document their limits
// ("max: 50"). An oversized limit still has an obvious meaning; a negative one does not.
func cappedIntArg(args map[string]any, key string, fallback, minimum, maximum int) (int, error) {
	value, err := intArg(args, key, fallback)
	if err != nil {
		return 0, err
	}
	if value < minimum {
		return 0, fmt.Errorf("%w %s %d (must be at least %d)", errInvalid, key, value, minimum)
	}
	return min(value, maximum), nil
}

// intArg returns the integer argument stored under key, or fallback when the argument is
// absent. JSON numbers decode into float64; any other type is a caller mistake and is
// rejected rather than replaced by the default. A fractional or out-of-range number is
// rejected too: truncating it would silently answer a different question, and a float
// beyond int range converts to an implementation-defined value.
func intArg(args map[string]any, key string, fallback int) (int, error) {
	raw, ok := args[key]
	if !ok {
		return fallback, nil
	}
	value, isFloat := raw.(float64)
	if !isFloat {
		return 0, fmt.Errorf("%w %s: expected a number, got %T", errInvalid, key, raw)
	}
	if value != math.Trunc(value) {
		return 0, fmt.Errorf("%w %s: expected a whole number, got %v", errInvalid, key, value)
	}
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmt.Errorf("%w %s: %v is out of range (accepted: %d to %d)",
			errInvalid, key, value, math.MinInt32, math.MaxInt32)
	}
	return int(value), nil
}
