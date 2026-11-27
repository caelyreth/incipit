package main

import "testing"

func TestIsModuleRootHandlesLFAndCRLF(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		gomod string
		want  bool
	}{
		{name: "unix lf", gomod: "module github.com/caelyreth/incipit\n\ngo 1.27.3\n", want: true},
		{name: "windows crlf", gomod: "module github.com/caelyreth/incipit\r\n\r\ngo 1.27.3\r\n", want: true},
		{name: "no trailing newline", gomod: "module github.com/caelyreth/incipit", want: true},
		{name: "other module", gomod: "module github.com/caelyreth/incipit-other\r\n", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isModuleRoot([]byte(tc.gomod), "github.com/caelyreth/incipit"); got != tc.want {
				t.Fatalf("isModuleRoot(%q) = %v, want %v", tc.gomod, got, tc.want)
			}
		})
	}
}
