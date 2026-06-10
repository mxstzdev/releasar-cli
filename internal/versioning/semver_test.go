package versioning

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSemVer_Name(t *testing.T) {
	assert.Equal(t, "semver", SemVer{}.Name())
}

func TestSemVer_Parse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"standard", "1.2.3", "1.2.3", false},
		{"v prefix", "v1.2.3", "1.2.3", false},
		{"zeros", "0.0.0", "0.0.0", false},
		{"pre-release suffix stripped", "1.2.3-alpha.1", "1.2.3", false},
		{"missing patch", "1.2", "", true},
		{"non-numeric major", "x.2.3", "", true},
		{"non-numeric minor", "1.x.3", "", true},
		{"non-numeric patch", "1.2.x", "", true},
		{"empty", "", "", true},
	}

	s := SemVer{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := s.Parse(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, v.String())
		})
	}
}

func TestSemVer_IsInitial(t *testing.T) {
	s := SemVer{}

	v, err := s.Parse("0.0.0")
	require.NoError(t, err)
	assert.True(t, v.IsInitial())

	v, err = s.Parse("0.0.1")
	require.NoError(t, err)
	assert.False(t, v.IsInitial())
}

func TestSemVer_Increment(t *testing.T) {
	tests := []struct {
		name  string
		input string
		bump  BumpKind
		want  string
	}{
		{"major resets minor and patch", "1.2.3", BumpMajor, "2.0.0"},
		{"minor resets patch", "1.2.3", BumpMinor, "1.3.0"},
		{"patch", "1.2.3", BumpPatch, "1.2.4"},
		{"none returns same", "1.2.3", BumpNone, "1.2.3"},
	}

	s := SemVer{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := s.Parse(tt.input)
			require.NoError(t, err)
			got, err := v.Increment(tt.bump)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.String())
		})
	}
}

func TestSemVer_Compare(t *testing.T) {
	tests := []struct {
		name  string
		a, b  string
		want  int
	}{
		{"equal", "1.2.3", "1.2.3", 0},
		{"greater major", "2.0.0", "1.9.9", 1},
		{"lesser major", "1.0.0", "2.0.0", -1},
		{"greater minor", "1.3.0", "1.2.9", 1},
		{"lesser minor", "1.2.0", "1.3.0", -1},
		{"greater patch", "1.2.4", "1.2.3", 1},
		{"lesser patch", "1.2.3", "1.2.4", -1},
	}

	s := SemVer{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := s.Parse(tt.a)
			require.NoError(t, err)
			b, err := s.Parse(tt.b)
			require.NoError(t, err)
			assert.Equal(t, tt.want, a.Compare(b))
		})
	}
}

func TestSemVer_Convenience(t *testing.T) {
	s := SemVer{}
	a, _ := s.Parse("1.2.3")
	b, _ := s.Parse("1.2.4")

	assert.True(t, a.LessThan(b))
	assert.True(t, b.GreaterThan(a))
	assert.True(t, a.Equal(a))
	assert.False(t, a.Equal(b))
}
