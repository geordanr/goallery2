package gallery

import "testing"

func TestJoinPathComponents(t *testing.T) {
	tests := []struct {
		components []string
		want       string
	}{
		{[]string{"geordan", "ax2004", "124_2423_IMG.jpg"}, "geordan/ax2004/124_2423_IMG.jpg"},
		{[]string{"albums", "foo"}, "albums/foo"},
		{[]string{"single"}, "single"},
		{[]string{}, ""},
	}

	for _, tc := range tests {
		if got := joinPathComponents(tc.components); got != tc.want {
			t.Errorf("joinPathComponents(%v) = %q, want %q", tc.components, got, tc.want)
		}
	}
}
