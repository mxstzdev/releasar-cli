package versioning

import (
	"fmt"
	"strconv"
	"strings"
)

// SemVer is a Scheme implementation for Semantic Versioning (https://semver.org).
type SemVer struct{}

func (s SemVer) Name() string {
	return "semver"
}

func (s SemVer) Parse(str string) (Version, error) {
	str = strings.TrimPrefix(str, "v")

	parts := strings.SplitN(str, ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("parsing semver %q: expected MAJOR.MINOR.PATCH", str)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("parsing semver %q: invalid major: %w", str, err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("parsing semver %q: invalid minor: %w", str, err)
	}

	// strip pre-release suffix from patch (e.g. "3-alpha.1" → "3")
	patchStr := strings.SplitN(parts[2], "-", 2)[0]
	patch, err := strconv.Atoi(patchStr)
	if err != nil {
		return nil, fmt.Errorf("parsing semver %q: invalid patch: %w", str, err)
	}

	return &semverVersion{scheme: s, major: major, minor: minor, patch: patch}, nil
}

type semverVersion struct {
	scheme SemVer
	major  int
	minor  int
	patch  int
}

func (v *semverVersion) Scheme() Scheme { return v.scheme }

func (v *semverVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}

func (v *semverVersion) IsInitial() bool {
	return v.major == 0 && v.minor == 0 && v.patch == 0
}

func (v *semverVersion) Increment(bump BumpKind) (Version, error) {
	switch bump {
	case BumpMajor:
		return &semverVersion{scheme: v.scheme, major: v.major + 1}, nil
	case BumpMinor:
		return &semverVersion{scheme: v.scheme, major: v.major, minor: v.minor + 1}, nil
	case BumpPatch:
		return &semverVersion{scheme: v.scheme, major: v.major, minor: v.minor, patch: v.patch + 1}, nil
	case BumpNone:
		return &semverVersion{scheme: v.scheme, major: v.major, minor: v.minor, patch: v.patch}, nil
	default:
		return nil, fmt.Errorf("semver: unknown bump kind %d", bump)
	}
}

func (v *semverVersion) Compare(other Version) int {
	o, ok := other.(*semverVersion)
	if !ok {
		return -1
	}
	if v.major != o.major {
		return cmp(v.major, o.major)
	}
	if v.minor != o.minor {
		return cmp(v.minor, o.minor)
	}
	return cmp(v.patch, o.patch)
}

func (v *semverVersion) Equal(other Version) bool       { return v.Compare(other) == 0 }
func (v *semverVersion) LessThan(other Version) bool    { return v.Compare(other) < 0 }
func (v *semverVersion) GreaterThan(other Version) bool { return v.Compare(other) > 0 }

// cmp returns -1, 0, or 1 for integer comparison.
func cmp(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
