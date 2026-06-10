package versioning

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalVer_Name(t *testing.T) {
	assert.Equal(t, "calver", CalVer{}.Name())
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"YYYY.0M.MICRO", "2024.06.3", "YYYY.0M.MICRO", false},
		{"YYYY.MM.MICRO", "2024.6.3", "YYYY.MM.MICRO", false},
		{"YYYY.0M", "2024.06", "YYYY.0M", false},
		{"YYYY.MM", "2024.6", "YYYY.MM", false},
		{"unrecognised", "not-a-version", "", true},
		{"too many segments", "2024.06.3.1", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := detectFormat(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, f.name)
		})
	}
}

func TestCalVer_Parse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"YYYY.0M.MICRO", "2024.06.3", "2024.06.3", false},
		{"YYYY.MM.MICRO", "2024.6.3", "2024.6.3", false},
		{"YYYY.0M", "2024.06", "2024.06", false},
		{"YYYY.MM", "2024.6", "2024.6", false},
		{"v prefix stripped", "v2024.06.3", "2024.06.3", false},
		{"micro zero", "2024.06.0", "2024.06.0", false},
		{"unrecognised format", "not-a-version", "", true},
		{"non-numeric year", "abcd.06.3", "", true},
		{"non-numeric month", "2024.xx.3", "", true},
		{"non-numeric micro", "2024.06.x", "", true},
	}

	s := CalVer{}
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

func TestCalVer_ParseWithExplicitFormat(t *testing.T) {
	s := CalVer{Format: knownCalVerFormats[0]} // YYYY.0M.MICRO
	v, err := s.Parse("2024.06.3")
	require.NoError(t, err)
	assert.Equal(t, "2024.06.3", v.String())
}

func TestCalVer_IsInitial(t *testing.T) {
	s := CalVer{}

	v, err := s.Parse("2024.06.0")
	require.NoError(t, err)
	assert.False(t, v.IsInitial())
}

func TestCalVer_Increment(t *testing.T) {
	fixedDate := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	calverNow = func() time.Time { return fixedDate }
	t.Cleanup(func() { calverNow = time.Now })

	tests := []struct {
		name  string
		input string
		bump  BumpKind
		want  string
	}{
		{"same month increments micro", "2024.06.2", BumpPatch, "2024.06.3"},
		{"new year resets micro", "2023.06.5", BumpPatch, "2024.06.0"},
		{"new month resets micro", "2024.05.5", BumpPatch, "2024.06.0"},
		{"major bump updates date", "2023.01.0", BumpMajor, "2024.06.0"},
		{"minor bump updates date", "2023.01.0", BumpMinor, "2024.06.0"},
		{"none returns unchanged", "2024.06.2", BumpNone, "2024.06.2"},
		{"two-segment same month", "2024.06", BumpPatch, "2024.06"},
		{"two-segment new month", "2024.05", BumpPatch, "2024.06"},
	}

	s := CalVer{}
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

func TestCalVer_Compare(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"equal", "2024.06.3", "2024.06.3", 0},
		{"greater year", "2025.01.0", "2024.12.9", 1},
		{"lesser year", "2024.01.0", "2025.01.0", -1},
		{"greater month", "2024.07.0", "2024.06.9", 1},
		{"lesser month", "2024.05.0", "2024.06.0", -1},
		{"greater micro", "2024.06.4", "2024.06.3", 1},
		{"lesser micro", "2024.06.3", "2024.06.4", -1},
	}

	s := CalVer{}
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

func TestCalVer_Convenience(t *testing.T) {
	s := CalVer{}
	a, _ := s.Parse("2024.06.3")
	b, _ := s.Parse("2024.06.4")

	assert.True(t, a.LessThan(b))
	assert.True(t, b.GreaterThan(a))
	assert.True(t, a.Equal(a))
	assert.False(t, a.Equal(b))
}
