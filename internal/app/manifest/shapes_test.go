package manifest

import (
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/caelyreth/neverland/internal/app/storage"
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/engine"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// The invariants again, from documents typing seldom builds: the shapes a
// notebook comes to hold over months of editing (adjacent lists of one
// style, empty and deep lists, tasks, quotes holding lists, figures,
// tables, math, code), with every command of the catalog among the
// intents. A trace that fails names its seed; NEVERLAND_SEEDS runs more.
// NEVERLAND_NOTEBOOK names a notebook folder whose pages, opened from a
// copy, start traces too.

// shapeWords are what generated text is drawn from.
var shapeWords = []string{"", "tide", "moon and sea", "中文", "**held**", "a *b* c", "x", "long line of words that goes on"}

// randomBlocks is up to n blocks of the page's flow, depth deep at most.
func randomBlocks(rng *rand.Rand, n, depth int) []*doc.Node {
	s := profile.Schema()
	text := func() []doc.Item {
		if w := shapeWords[rng.IntN(len(shapeWords))]; w != "" {
			return []doc.Item{doc.TextItem(w, nil)}
		}
		return nil
	}
	var out []*doc.Node
	for range 1 + rng.IntN(n) {
		switch k := rng.IntN(100); {
		case k < 22:
			out = append(out, profile.NewParagraph(text()...))
		case k < 32:
			level := field(profile.FieldLevel, 1+rng.IntN(6), 1)
			out = append(out, vocab.NewBlock(s, profile.Heading, level, text()...))
		case k < 62:
			// Lists come in runs: two or three in a row, often of one style,
			// as the notebook that crashed on 2026-11-23 held.
			style := profile.ListStyle(rng.IntN(2))
			for range 1 + rng.IntN(3) {
				if rng.IntN(3) == 0 {
					style = profile.ListStyle(rng.IntN(2))
				}
				out = append(out, randomList(rng, style, depth))
			}
		case k < 70:
			if depth > 0 {
				out = append(out, doc.NewNode(profile.Quote, doc.Attrs{}, doc.BlockSlot(randomBlocks(rng, 3, depth-1)...)))
			}
		case k < 76:
			out = append(out, vocab.NewBlock(s, profile.Code, doc.Attrs{}, text()...))
		case k < 80:
			out = append(out, vocab.NewBlock(s, profile.Math, doc.Attrs{}, doc.TextItem("x^2", nil)))
		case k < 84:
			out = append(out, vocab.NewBlock(s, profile.Divider, doc.Attrs{}))
		case k < 92:
			caption := ""
			if rng.IntN(2) == 0 {
				caption = "a caption"
			}
			out = append(out, profile.NewFigure(s, profile.Picture{Media: "m", Width: 4, Height: 3}, caption))
		default:
			out = append(out, profile.NewTable(1+rng.IntN(3), 1+rng.IntN(3)))
		}
	}
	return out
}

// randomList is a list of style, its items tasks or not, some empty, some
// holding lists or paragraphs of their own.
func randomList(rng *rand.Rand, style profile.ListStyle, depth int) *doc.Node {
	attrs := field(profile.FieldListStyle, int(style), int(profile.Bullet))
	var items []*doc.Node
	for range 1 + rng.IntN(4) {
		var head []doc.Item
		if w := shapeWords[rng.IntN(len(shapeWords))]; w != "" {
			head = append(head, doc.TextItem(w, nil))
		}
		var body []*doc.Node
		if depth > 0 && rng.IntN(4) == 0 {
			body = append(body, randomList(rng, profile.ListStyle(rng.IntN(2)), depth-1))
		}
		if depth > 0 && rng.IntN(8) == 0 {
			body = append(body, profile.NewParagraph(doc.TextItem("more", nil)))
		}
		task := field(profile.FieldTask, rng.IntN(3), int(profile.NoTask))
		items = append(items, doc.NewNode(profile.Item, task, doc.InlineSlot(head...), doc.BlockSlot(body...)))
	}
	return doc.NewNode(profile.List, attrs, doc.BlockSlot(items...))
}

// field is attrs holding num at v, or none at its default, which a
// document never stores.
func field(num uint16, v, def int) doc.Attrs {
	if v == def {
		return doc.Attrs{}
	}
	return doc.Attrs{}.With(num, doc.Int(int64(v)))
}

// randomPage is a page holding randomBlocks, valid by construction.
func randomPage(t *testing.T, rng *rand.Rand, seed uint64) *doc.Snapshot {
	t.Helper()
	empty, err := profile.EmptyPage(1, seed, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	tx := empty.Begin(doc.Origin{Class: doc.OriginCommand})
	if err := tx.Apply(doc.Insert{Parent: 1, Nodes: randomBlocks(rng, 10, 2)}); err != nil {
		t.Fatal(err)
	}
	snap, err := tx.Commit()
	if err != nil {
		t.Fatalf("seed %d: the generated page is not valid: %v", seed, err)
	}
	return snap
}

// commandIntent draws as randomIntent does, and a fifth of the time runs
// any command of the catalog.
func commandIntent(commands []vocab.CommandID) func(*rand.Rand, *engine.Engine) vocab.Intent {
	return func(rng *rand.Rand, e *engine.Engine) vocab.Intent {
		if rng.IntN(5) == 0 {
			return vocab.Command{ID: commands[rng.IntN(len(commands))]}
		}
		return randomIntent(rng, e)
	}
}

func TestRandomShapesKeepTheInvariants(t *testing.T) {
	seeds := 24
	if s := os.Getenv("NEVERLAND_SEEDS"); s != "" {
		seeds, _ = strconv.Atoi(s)
	}
	length := 120
	if testing.Short() {
		seeds, length = 6, 60
	}
	draw := catalogIntent(t)
	for seed := range seeds {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(uint64(seed), 7))
			runTraceFrom(t, uint64(seed), length, randomPage(t, rng, uint64(seed)), draw)
		})
	}
}

// catalogIntent is commandIntent over the whole catalog of prog.
func catalogIntent(t *testing.T) func(*rand.Rand, *engine.Engine) vocab.Intent {
	t.Helper()
	prog, err := Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	var commands []vocab.CommandID
	for _, c := range prog.Catalog().Commands {
		commands = append(commands, c.ID)
	}
	return commandIntent(commands)
}

// TestANotebooksPagesKeepTheInvariants runs traces from every page of the
// notebook NEVERLAND_NOTEBOOK names, as they are: the shapes months of
// writing left. The notebook is copied first; opening takes a backup.
func TestANotebooksPagesKeepTheInvariants(t *testing.T) {
	src := os.Getenv("NEVERLAND_NOTEBOOK")
	if src == "" {
		t.Skip("NEVERLAND_NOTEBOOK names no notebook")
	}
	dir := t.TempDir()
	for _, name := range []string{"notebook.db", "notebook.db-wal", "notebook.db-shm"} {
		if err := copyFile(filepath.Join(src, name), filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	contents, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	draw := catalogIntent(t)
	for _, page := range contents.Pages {
		snap, err := store.LoadPage(page.Key, profile.Schema(), doc.NewInterner())
		if err != nil {
			t.Errorf("page %d: %v", page.Key, err)
			continue
		}
		if snap == nil {
			continue
		}
		for seed := range 4 {
			t.Run(fmt.Sprintf("page%d/%d", page.Key, seed), func(t *testing.T) {
				runTraceFrom(t, uint64(seed), 80, snap, draw)
			})
		}
	}
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
