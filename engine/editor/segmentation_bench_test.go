package editor

import "testing"

func BenchmarkWordLikeRanges(b *testing.B) {
	for _, test := range []struct {
		name string
		text string
	}{
		{"Latin", "Lorem ipsum dolor sit amet, consectetur adipiscing elit."},
		{"CJK", "南京市长江大桥，我喜欢编程和中文测试。日本語の文章です。"},
		{"ThaiFallback", "ก่อนภาษาไทยหลัง"},
	} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = WordLikeRanges(test.text)
			}
		})
	}
}
