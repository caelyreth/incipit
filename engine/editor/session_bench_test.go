package editor

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
)

func benchmarkProse(size int) string {
	const unit = "Long prose 中文 mixed with ordinary words and punctuation.\n"
	return strings.Repeat(unit, max(1, size/len(unit)))
}

func benchmarkSession(b *testing.B, text string, heavy bool) *Session {
	b.Helper()
	document, blocks, err := NewBlockDocument(text)
	if err != nil {
		b.Fatal(err)
	}
	node := blocks[len(blocks)-1]
	catalog, err := EditorPolicyCatalog()
	if err != nil {
		b.Fatal(err)
	}
	runtime, err := kernel.NewPolicyRuntime(document, catalog)
	if err != nil {
		b.Fatal(err)
	}
	session, err := NewSession(runtime, node, &sessionClock{now: time.Unix(1_700_000_000, 0)})
	if err != nil {
		b.Fatal(err)
	}
	if heavy {
		if _, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
			configuration.SetEnabled(CJKPolicyID, true)
			configuration.SetEnabled(TypographyPolicyID, true)
			return nil
		}); err != nil {
			b.Fatal(err)
		}
	}
	end := session.Projection().UTF16Len()
	if err := session.SelectProjected(end, end); err != nil {
		b.Fatal(err)
	}
	return session
}

func BenchmarkSessionTypingLongProse(b *testing.B) {
	for _, size := range []int{8 << 10, 64 << 10, 256 << 10} {
		b.Run(sizeName(size), func(b *testing.B) {
			session := benchmarkSession(b, benchmarkProse(size), false)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := session.replaceSelection("x", kernel.EventOriginTyping); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSessionTypingHeavyPolicies(b *testing.B) {
	for _, size := range []int{8 << 10, 64 << 10, 256 << 10} {
		b.Run(sizeName(size), func(b *testing.B) {
			session := benchmarkSession(b, benchmarkProse(size), true)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := session.replaceSelection("x", kernel.EventOriginTyping); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func sizeName(size int) string {
	switch size {
	case 8 << 10:
		return "8KiB"
	case 64 << 10:
		return "64KiB"
	case 256 << 10:
		return "256KiB"
	default:
		return "other"
	}
}
