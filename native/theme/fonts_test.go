package theme

import "testing"

func TestReferenceProseFontRegisters(t *testing.T) {
	if err := RegisterFonts(); err != nil {
		t.Fatal(err)
	}
}
