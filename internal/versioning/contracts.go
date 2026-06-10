package versioning

// Scheme represents a versioning scheme.
type Scheme interface {
	Name() string
	Parse(s string) (Version, error)
}

// Version represents a parsed project version.
type Version interface {
	Scheme() Scheme
	String() string
	IsInitial() bool
	Increment(bump BumpKind) (Version, error)
	// Compare returns 0 if equal, -1 if other is greater, 1 if v is greater.
	Compare(other Version) int
	Equal(other Version) bool
	LessThan(other Version) bool
	GreaterThan(other Version) bool
}
