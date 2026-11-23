// Command cjkdict writes internal/text/wordseg/cjk_dictionary.bin, the word
// segmenter's Chinese and Japanese dictionary, from ICU's own word list:
// it downloads cjdict.txt at the ICU release pinned below, verifies it,
// and indexes it in Neverland's format, which it verifies too, so that
// every build holds the same bytes. A dictionary already written and
// verified is kept. Run it from the repository's root, as
// scripts/fetch-assets.sh does:
//
//	go tool cjkdict
//
// The format (wordseg's cjk.go reads it): "NVCJ", a little-endian uint32
// version, the uint32 count of words, count+1 uint32 offsets of the words
// in the text that follows the costs, a byte of cost for each word, and
// the words, in byte order, each once.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// source is ICU release 78.3's cjdict.txt, and its SHA-256.
const (
	source       = "https://raw.githubusercontent.com/unicode-org/icu/release-78.3/icu4c/source/data/brkitr/dictionaries/cjdict.txt"
	sourceSHA256 = "e73fd72048981d0cc13e9dc436a7eaba07ffb6eff58c8a59dc75c1df746663a0"
)

// The index's version, and the SHA-256 of the index of source: a change to
// either is a change to the format, which wordseg reads.
const (
	version     = 1
	indexSHA256 = "35ca601ac1c6fbc9900ee1d692c32c2a958254f6a9f31c77e841ee09863db577"
)

func main() {
	out := flag.String("o", "internal/text/wordseg/cjk_dictionary.bin", "the dictionary to write")
	from := flag.String("src", "", "a cjdict.txt to read in place of the pinned download")
	flag.Parse()
	if have, err := os.ReadFile(*out); err == nil && sum(have) == indexSHA256 {
		log.Printf("%s is already present and verified", *out)
		return
	}
	text, err := read(*from)
	if err != nil {
		log.Fatal(err)
	}
	index, err := build(text)
	if err != nil {
		log.Fatal(err)
	}
	if got := sum(index); got != indexSHA256 {
		log.Fatalf("the index is not the one pinned: %s, want %s", got, indexSHA256)
	}
	if err := os.WriteFile(*out, index, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s", *out)
}

// read is the word list at path, or the pinned one downloaded, verified.
func read(path string) ([]byte, error) {
	var data []byte
	var err error
	if path != "" {
		data, err = os.ReadFile(path)
	} else {
		data, err = download(source)
	}
	if err != nil {
		return nil, err
	}
	if got := sum(data); got != sourceSHA256 {
		return nil, fmt.Errorf("cjdict.txt is not the one pinned: %s, want %s", got, sourceSHA256)
	}
	return data, nil
}

// download is what u answers, asked again while the network fails.
func download(u string) ([]byte, error) {
	var err error
	for try := range 5 {
		if try > 0 {
			time.Sleep(time.Duration(1<<try) * time.Second)
		}
		var resp *http.Response
		if resp, err = http.Get(u); err != nil {
			continue
		}
		data, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		switch {
		case rerr != nil:
			err = rerr
		case resp.StatusCode != http.StatusOK:
			err = fmt.Errorf("%s answered %s", u, resp.Status)
		default:
			return data, nil
		}
	}
	return nil, err
}

// build indexes the word list: each line a word, a tab and its cost;
// comment and blank lines left out.
func build(text []byte) ([]byte, error) {
	type entry struct {
		word string
		cost byte
	}
	var words []entry
	lines := bufio.NewScanner(bytes.NewReader(text))
	for lines.Scan() {
		line := strings.TrimPrefix(lines.Text(), "\ufeff")
		if t := strings.TrimSpace(line); t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		word, cost, ok := strings.Cut(line, "\t")
		if !ok {
			return nil, fmt.Errorf("a line with no cost: %q", line)
		}
		n, err := strconv.Atoi(strings.TrimSpace(cost))
		if err != nil || n < 0 || n > 255 {
			return nil, fmt.Errorf("a cost not of a byte: %q", line)
		}
		words = append(words, entry{word, byte(n)})
	}
	if err := lines.Err(); err != nil {
		return nil, err
	}
	slices.SortStableFunc(words, func(a, b entry) int { return strings.Compare(a.word, b.word) })
	words = slices.CompactFunc(words, func(a, b entry) bool { return a.word == b.word })
	var b bytes.Buffer
	u32 := func(v int) { b.Write(binary.LittleEndian.AppendUint32(nil, uint32(v))) }
	b.WriteString("NVCJ")
	u32(version)
	u32(len(words))
	at := 0
	for _, w := range words {
		u32(at)
		at += len(w.word)
	}
	u32(at)
	for _, w := range words {
		b.WriteByte(w.cost)
	}
	for _, w := range words {
		b.WriteString(w.word)
	}
	return b.Bytes(), nil
}

func sum(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}
