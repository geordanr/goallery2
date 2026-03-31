package gallery

import "testing"

func TestDerivativeCachePath(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{10047, "cache/derivative/1/0/10047.dat"},
		{25000, "cache/derivative/2/5/25000.dat"},
		{9, "cache/derivative/0/9/9.dat"},
		{1, "cache/derivative/0/1/1.dat"},
	}

	for _, tc := range tests {
		d := Derivative{ID: tc.id}
		if got := d.CachePath(); got != tc.want {
			t.Errorf("CachePath() for ID %d = %q, want %q", tc.id, got, tc.want)
		}
	}
}
