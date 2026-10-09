package git

import "testing"

func TestParseGitVersion(t *testing.T) {
	tests := []struct {
		raw     string
		want    GitVersion
		wantErr bool
	}{
		{raw: "git version 2.43.0", want: GitVersion{Major: 2, Minor: 43, Patch: 0}},
		{raw: "git version 2.43.0.windows.1", want: GitVersion{Major: 2, Minor: 43, Patch: 0}},
		{raw: "git version 2.39.3 (Apple Git-146)", want: GitVersion{Major: 2, Minor: 39, Patch: 3}},
		{raw: "git version 2.38", want: GitVersion{Major: 2, Minor: 38, Patch: 0}},
		{raw: "git version 2.45.0-rc1", want: GitVersion{Major: 2, Minor: 45, Patch: 0}},
		{raw: "git version 2.45.0.rc1", want: GitVersion{Major: 2, Minor: 45, Patch: 0}},
		{raw: "not git", wantErr: true},
		{raw: "git version banana", wantErr: true},
		{raw: "", wantErr: true},
	}

	for _, tt := range tests {
		got, err := ParseGitVersion(tt.raw)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseGitVersion(%q) = %v, want an error", tt.raw, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseGitVersion(%q): %v", tt.raw, err)
			continue
		}
		if got.Major != tt.want.Major || got.Minor != tt.want.Minor || got.Patch != tt.want.Patch {
			t.Errorf("ParseGitVersion(%q) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}

func TestGitVersionAtLeast(t *testing.T) {
	v := func(a, b, c int) GitVersion { return GitVersion{Major: a, Minor: b, Patch: c} }

	tests := []struct {
		have, need GitVersion
		want       bool
	}{
		{v(2, 43, 0), v(2, 38, 0), true},
		{v(2, 38, 0), v(2, 38, 0), true},
		{v(2, 37, 9), v(2, 38, 0), false},
		{v(3, 0, 0), v(2, 38, 0), true},
		{v(1, 99, 99), v(2, 38, 0), false},
	}
	for _, tt := range tests {
		if got := tt.have.AtLeast(tt.need); got != tt.want {
			t.Errorf("%v.AtLeast(%v) = %v, want %v", tt.have, tt.need, got, tt.want)
		}
	}
}

func TestInstalledVersionIsReadable(t *testing.T) {
	v, err := InstalledVersion()
	if err != nil {
		t.Skipf("git not available: %v", err)
	}
	if v.Major == 0 {
		t.Errorf("InstalledVersion() = %+v, want a real version", v)
	}
}
