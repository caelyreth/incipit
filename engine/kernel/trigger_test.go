package kernel

import "testing"

func TestTriggerPatternsRoundTripAndValidate(t *testing.T) {
	t.Parallel()
	for _, pattern := range []string{"text:input", "text:*", "*", "state:*", "界😀*", "界😀"} {
		trigger, err := TryParseTrigger(pattern)
		if err != nil || trigger.String() != pattern {
			t.Fatalf("%q: trigger=%v err=%v", pattern, trigger, err)
		}
	}
	for _, pattern := range []string{"", "text:*:bad", "text:**", "**", "界*😀"} {
		if _, err := TryParseTrigger(pattern); err == nil {
			t.Fatalf("%q accepted", pattern)
		}
	}
}

func TestTriggerMatchingAndOverlap(t *testing.T) {
	t.Parallel()
	patterns := []string{"text:input", "text:*", "text:in*", "*", "other", "界*", "界😀"}
	keys := []string{"", "text:", "text:input", "text:insert", "text:other", "other", "界", "界😀", "none"}
	for _, pattern := range patterns {
		trigger := ParseTrigger(pattern)
		for _, key := range keys {
			want := pattern == key
			if suffix, ok := cutSuffixStar(pattern); ok {
				want = len(key) >= len(suffix) && key[:len(suffix)] == suffix
			}
			if got := trigger.Matches(key); got != want {
				t.Fatalf("%q matches %q = %v, want %v", pattern, key, got, want)
			}
		}
		for _, other := range patterns {
			late, early := ParseTrigger(pattern), ParseTrigger(other)
			overlap := false
			for _, key := range keys {
				if late.Matches(key) && early.Matches(key) {
					overlap = true
					break
				}
			}
			if got := late.Overlaps(early); got != overlap {
				t.Fatalf("%q overlaps %q = %v, want %v", pattern, other, got, overlap)
			}
		}
	}
}

func cutSuffixStar(pattern string) (string, bool) {
	if pattern == "state:*" || pattern == "" || pattern[len(pattern)-1] != '*' {
		return "", false
	}
	return pattern[:len(pattern)-1], true
}
