package versioning

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var knownCalVerFormats = []calverFormat{
	{
		name:     "YYYY.0M.MICRO",
		pattern:  regexp.MustCompile(`^\d{4}\.\d{2}\.\d+$`),
		segments: []string{"YYYY", "0M", "MICRO"},
	},
	{
		name:     "YYYY.MM.MICRO",
		pattern:  regexp.MustCompile(`^\d{4}\.\d\.\d+$`),
		segments: []string{"YYYY", "MM", "MICRO"},
	},
	{
		name:     "YYYY.0M",
		pattern:  regexp.MustCompile(`^\d{4}\.\d{2}$`),
		segments: []string{"YYYY", "0M"},
	},
	{
		name:     "YYYY.MM",
		pattern:  regexp.MustCompile(`^\d{4}\.\d$`),
		segments: []string{"YYYY", "MM"},
	},
}

// CalVer is a Scheme implemenation for a release calendar based Versioning (https://calver.org/).
type CalVer struct {
	Format calverFormat
}

func (s CalVer) Name() string {
	return "calver"
}

func (s CalVer) Parse(str string) (Version, error) {
	str = strings.TrimPrefix(str, "v")

	format := s.Format
	if format.pattern == nil {
		var err error
		format, err = detectFormat(str)
		if err != nil {
			return nil, fmt.Errorf("parsing calver %q: %w", str, err)
		}
	}

	parts := strings.SplitN(str, ".", 3)

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("parsing calver %q: invalid first segment: %w", str, err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("parsing calver %q: invalid second segment: %w", str, err)
	}

	micro := 0
	if len(parts) == 3 {
		micro, err = strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("parsing calver %q: invalid micro segment: %w", str, err)
		}
	}

	return &calverVersion{
		scheme: s,
		format: format,
		major:  major,
		minor:  minor,
		micro:  micro,
	}, nil
}

func detectFormat(str string) (calverFormat, error) {
	for _, f := range knownCalVerFormats {
		if f.pattern.MatchString(str) {
			return f, nil
		}
	}
	return calverFormat{}, fmt.Errorf("no known calver format matched")
}

type calverFormat struct {
	name     string
	pattern  *regexp.Regexp
	segments []string // e.g. ["YYYY", "0M", "MICRO"]
}

type calverVersion struct {
	scheme CalVer
	format calverFormat
	major  int
	minor  int
	micro  int
}

// calverNow is the clock used by Increment; override in tests for determinism.
var calverNow = time.Now

func (v *calverVersion) Scheme() Scheme { return v.scheme }

func (v *calverVersion) String() string {
	seg := v.format.segments
	switch len(seg) {
	case 3:
		if seg[1] == "0M" {
			return fmt.Sprintf("%d.%02d.%d", v.major, v.minor, v.micro)
		}
		return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.micro)
	default:
		if seg[1] == "0M" {
			return fmt.Sprintf("%d.%02d", v.major, v.minor)
		}
		return fmt.Sprintf("%d.%d", v.major, v.minor)
	}
}

func (v *calverVersion) IsInitial() bool {
	return v.major == 0 && v.minor == 0 && v.micro == 0
}

func (v *calverVersion) Increment(bump BumpKind) (Version, error) {
	if bump == BumpNone {
		return &calverVersion{scheme: v.scheme, format: v.format, major: v.major, minor: v.minor, micro: v.micro}, nil
	}

	t := calverNow()
	year, month := t.Year(), int(t.Month())

	micro := 0
	if year == v.major && month == v.minor {
		micro = v.micro + 1
	}

	return &calverVersion{
		scheme: v.scheme,
		format: v.format,
		major:  year,
		minor:  month,
		micro:  micro,
	}, nil
}

func (v *calverVersion) Compare(other Version) int {
	o, ok := other.(*calverVersion)
	if !ok {
		return -1
	}
	if v.major != o.major {
		return cmp(v.major, o.major)
	}
	if v.minor != o.minor {
		return cmp(v.minor, o.minor)
	}
	return cmp(v.micro, o.micro)
}

func (v *calverVersion) Equal(other Version) bool       { return v.Compare(other) == 0 }
func (v *calverVersion) LessThan(other Version) bool    { return v.Compare(other) < 0 }
func (v *calverVersion) GreaterThan(other Version) bool { return v.Compare(other) > 0 }
