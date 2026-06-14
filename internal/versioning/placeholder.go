package versioning

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// placeholderPattern matches {{RLSR_LATEST}}, {{LATEST}}, {{RLSR_NEXT.MAJOR}}, {{NEXT.MINOR}}, etc.
// The RLSR_ prefix and the keyword are both case-insensitive.
var placeholderPattern = regexp.MustCompile(`(?i)\{\{(?:RLSR_)?(LATEST|NEXT\.(?:MAJOR|MINOR|PATCH))\}\}`)

// ReplacePlaceholders substitutes version placeholders in content with values derived from current.
// Unrecognised content inside {{ }} is left unchanged.
func ReplacePlaceholders(content string, current Version) (string, error) {
	var (
		b       strings.Builder
		lastEnd int
		err     error
	)

	matches := placeholderPattern.FindAllStringSubmatchIndex(content, -1)
	for _, m := range matches {
		b.WriteString(content[lastEnd:m[0]])

		key := strings.ToUpper(content[m[2]:m[3]])
		replacement, resolveErr := resolvePlaceholder(key, current)
		if resolveErr != nil {
			err = fmt.Errorf("resolving placeholder %q: %w", content[m[0]:m[1]], resolveErr)
			break
		}

		b.WriteString(replacement)
		lastEnd = m[1]
	}

	if err != nil {
		return "", err
	}

	b.WriteString(content[lastEnd:])
	return b.String(), nil
}

// ReplaceInFile replaces version placeholders in the file at path in-place,
// preserving the file's original permissions.
func ReplaceInFile(path string, current Version) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	result, err := ReplacePlaceholders(string(data), current)
	if err != nil {
		return fmt.Errorf("replacing placeholders in %s: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(result), info.Mode()); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

func resolvePlaceholder(key string, current Version) (string, error) {
	switch key {
	case "LATEST":
		return current.String(), nil
	case "NEXT.MAJOR":
		return incrementString(current, BumpMajor)
	case "NEXT.MINOR":
		return incrementString(current, BumpMinor)
	case "NEXT.PATCH":
		return incrementString(current, BumpPatch)
	default:
		return "", fmt.Errorf("unknown placeholder key %q", key)
	}
}

func incrementString(v Version, bump BumpKind) (string, error) {
	next, err := v.Increment(bump)
	if err != nil {
		return "", err
	}
	return next.String(), nil
}
