package doc

import "math/bits"

// entry is what the index knows of a node: its current version and where
// it stands.
type entry struct {
	node   *Node
	parent ID
	slot   int
	// root marks the page's root, which has no parent.
	root bool
}

// trie is a persistent hash array mapped trie from node id to entry. Ids
// are random, so their bits are the hash. Every change copies one path.
type trie struct {
	bitmap uint32
	slots  []trieSlot
	size   int
}

type trieSlot struct {
	sub   *trie
	key   ID
	value entry
	leaf  bool
}

const trieBits = 5

func (t *trie) Len() int {
	if t == nil {
		return 0
	}
	return t.size
}

func (t *trie) get(key ID) (entry, bool) {
	for shift := uint(0); t != nil; shift += trieBits {
		bit := uint32(1) << ((uint64(key) >> shift) & 31)
		if t.bitmap&bit == 0 {
			return entry{}, false
		}
		s := t.slots[bits.OnesCount32(t.bitmap&(bit-1))]
		if s.leaf {
			if s.key == key {
				return s.value, true
			}
			return entry{}, false
		}
		t = s.sub
		if shift > 64 {
			break
		}
	}
	return entry{}, false
}

func (t *trie) set(key ID, value entry) *trie {
	out, _ := t.setAt(key, value, 0)
	return out
}

func (t *trie) setAt(key ID, value entry, shift uint) (*trie, bool) {
	if t == nil {
		t = &trie{}
	}
	bit := uint32(1) << ((uint64(key) >> shift) & 31)
	i := bits.OnesCount32(t.bitmap & (bit - 1))
	c := &trie{bitmap: t.bitmap, size: t.size}
	if t.bitmap&bit == 0 {
		c.bitmap |= bit
		c.slots = make([]trieSlot, 0, len(t.slots)+1)
		c.slots = append(c.slots, t.slots[:i]...)
		c.slots = append(c.slots, trieSlot{key: key, value: value, leaf: true})
		c.slots = append(c.slots, t.slots[i:]...)
		c.size++
		return c, true
	}
	c.slots = append([]trieSlot(nil), t.slots...)
	s := t.slots[i]
	switch {
	case s.leaf && s.key == key:
		c.slots[i].value = value
		return c, false
	case s.leaf:
		sub := (&trie{}).setLeaf(s.key, s.value, shift+trieBits)
		sub, _ = sub.setAt(key, value, shift+trieBits)
		c.slots[i] = trieSlot{sub: sub}
		c.size++
		return c, true
	default:
		sub, added := s.sub.setAt(key, value, shift+trieBits)
		c.slots[i] = trieSlot{sub: sub}
		if added {
			c.size++
		}
		return c, added
	}
}

func (t *trie) setLeaf(key ID, value entry, shift uint) *trie {
	out, _ := t.setAt(key, value, shift)
	return out
}

func (t *trie) delete(key ID) *trie {
	out, _ := t.deleteAt(key, 0)
	return out
}

func (t *trie) deleteAt(key ID, shift uint) (*trie, bool) {
	if t == nil {
		return nil, false
	}
	bit := uint32(1) << ((uint64(key) >> shift) & 31)
	if t.bitmap&bit == 0 {
		return t, false
	}
	i := bits.OnesCount32(t.bitmap & (bit - 1))
	s := t.slots[i]
	if s.leaf {
		if s.key != key {
			return t, false
		}
		c := &trie{bitmap: t.bitmap &^ bit, size: t.size - 1}
		c.slots = make([]trieSlot, 0, len(t.slots)-1)
		c.slots = append(c.slots, t.slots[:i]...)
		c.slots = append(c.slots, t.slots[i+1:]...)
		return c, true
	}
	sub, removed := s.sub.deleteAt(key, shift+trieBits)
	if !removed {
		return t, false
	}
	c := &trie{bitmap: t.bitmap, size: t.size - 1, slots: append([]trieSlot(nil), t.slots...)}
	switch {
	case sub.size == 0:
		c.bitmap &^= bit
		c.slots = append(c.slots[:i], c.slots[i+1:]...)
	case sub.size == 1 && len(sub.slots) == 1 && sub.slots[0].leaf:
		c.slots[i] = sub.slots[0]
	default:
		c.slots[i] = trieSlot{sub: sub}
	}
	return c, true
}
