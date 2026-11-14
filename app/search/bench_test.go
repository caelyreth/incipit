package search

import (
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"
)

// corpus is a made-up notebook: pages of English drawn from a Zipf law
// over a vocabulary of made-up words, every fifth page Chinese.
func corpus(pages int) []Page {
	r := rand.New(rand.NewPCG(7, 11))
	vocab := make([]string, 20000)
	for i := range vocab {
		var b strings.Builder
		for range 3 + r.IntN(7) {
			b.WriteByte(byte('a' + r.IntN(26)))
		}
		vocab[i] = b.String()
	}
	vocab[0], vocab[1], vocab[2] = "budget", "review", "planning"
	zipf := rand.NewZipf(r, 1.1, 1, uint64(len(vocab)-1))
	hanzi := []rune("我们正在设计关系数据库的索引数据很重要今天会议讨论预算计划项目进度文档搜索结果标签文件夹智能列表时间字数用户界面动画细节性能")
	line := func(chinese bool) string {
		var b strings.Builder
		if chinese {
			for range 12 + r.IntN(20) {
				b.WriteRune(hanzi[r.IntN(len(hanzi))])
			}
			return b.String()
		}
		for i := range 12 {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(vocab[zipf.Uint64()])
		}
		return b.String()
	}
	out := make([]Page, pages)
	for p := range out {
		chinese := p%5 == 0
		var body []string
		for range 25 {
			body = append(body, line(chinese))
		}
		out[p] = Page{
			Key: int64(p + 1), Rev: 1, Title: line(false)[:24], Body: strings.Join(body, "\n"),
			Tags: []string{vocab[3+p%30]}, Groups: []int64{int64(2 + p%12)}, Kinds: []string{"link"},
			Created: now.Add(-time.Duration(r.IntN(700*24)) * time.Hour), Updated: now.Add(-time.Duration(r.IntN(300*24)) * time.Hour),
			Words: 300,
		}
	}
	return out
}

var (
	benchOnce  sync.Once
	benchIndex *Index
)

func benchNotebook(b *testing.B) *Index {
	benchOnce.Do(func() {
		x, err := Memory()
		if err != nil {
			b.Fatal(err)
		}
		pages := corpus(10000)
		for i := 0; i < len(pages); i += 250 {
			if err := x.Put(pages[i:min(i+250, len(pages))]...); err != nil {
				b.Fatal(err)
			}
		}
		benchIndex = x
	})
	return benchIndex
}

func BenchmarkSearch(b *testing.B) {
	x := benchNotebook(b)
	for _, text := range []string{
		"budget ", "budget review ", `"budget review"`, "budg", "budgte ", "数据库", "数",
		"budget $updated:30d $tag:" + "x", "$sort:updated", "",
	} {
		q := Parse(text)
		b.Run(text, func(b *testing.B) {
			for b.Loop() {
				if _, err := x.Search(q, Request{Env: env()}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkPutOnePage(b *testing.B) {
	x := benchNotebook(b)
	page := corpus(1)[0]
	page.Key = 42
	for b.Loop() {
		page.Rev++
		if err := x.Put(page); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMatcherFind(b *testing.B) {
	page := corpus(5)[1]
	m := NewMatcher(Parse("budget review planning"), false)
	for b.Loop() {
		m.Find(page.Body)
	}
}

func BenchmarkTokens(b *testing.B) {
	pages := corpus(5)
	text := pages[0].Body + "\n" + pages[1].Body
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		Tokens(text, Indexed)
	}
}
