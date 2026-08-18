package version

import "testing"

func TestResolve(t *testing.T) {
	tests := []struct {
		name         string
		buildVersion string
		want         string
	}{
		{name: "empty", want: "dev"},
		{name: "development build", buildVersion: "(devel)", want: "dev"},
		{name: "tagged module", buildVersion: "v0.8.1", want: "0.8.1"},
		{name: "bare module version", buildVersion: "0.8.1", want: "0.8.1"},
		{name: "single leading v", buildVersion: "vv0.8.1", want: "v0.8.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolve(tt.buildVersion); got != tt.want {
				t.Fatalf("resolve(%q) = %q, want %q", tt.buildVersion, got, tt.want)
			}
		})
	}
}
