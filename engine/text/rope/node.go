package rope

import "strings"

type node struct {
	left    *node
	right   *node
	text    string
	summary Summary
	height  int
}

func height(n *node) int {
	if n == nil {
		return 0
	}
	return n.height
}

func leaf(text string) *node {
	if text == "" {
		return nil
	}
	return &node{text: strings.Clone(text), summary: summarize(text), height: 1}
}

func branch(left, right *node) *node {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	return &node{
		left:    left,
		right:   right,
		summary: left.summary.add(right.summary),
		height:  1 + max(left.height, right.height),
	}
}

func balance(n *node) *node {
	if height(n.left)-height(n.right) > 1 {
		left := n.left
		if height(left.left) < height(left.right) {
			pivot := left.right
			left = branch(branch(left.left, pivot.left), pivot.right)
		}
		return branch(left.left, branch(left.right, n.right))
	}
	if height(n.right)-height(n.left) > 1 {
		right := n.right
		if height(right.right) < height(right.left) {
			pivot := right.left
			right = branch(pivot.left, branch(pivot.right, right.right))
		}
		return branch(branch(n.left, right.left), right.right)
	}
	return n
}

func join(left, right *node) *node {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	if left.left == nil && right.left == nil && left.summary.Bytes+right.summary.Bytes <= chunkBytes {
		return leaf(left.text + right.text)
	}
	if left.height > right.height+1 {
		return balance(branch(left.left, join(left.right, right)))
	}
	if right.height > left.height+1 {
		return balance(branch(join(left, right.left), right.right))
	}
	return branch(left, right)
}

func build(text string) *node {
	var leaves []*node
	for start := 0; start < len(text); {
		end := start
		for end < len(text) && end-start < chunkBytes {
			_, size := utf8DecodeRune(text[end:])
			end += size
		}
		leaves = append(leaves, leaf(text[start:end]))
		start = end
	}
	var assemble func(int, int) *node
	assemble = func(start, end int) *node {
		switch end - start {
		case 0:
			return nil
		case 1:
			return leaves[start]
		default:
			mid := (start + end) / 2
			return branch(assemble(start, mid), assemble(mid, end))
		}
	}
	return assemble(0, len(leaves))
}

func split(n *node, at int) (*node, *node) {
	if n == nil {
		return nil, nil
	}
	if at <= 0 {
		return nil, n
	}
	if at >= n.summary.Bytes {
		return n, nil
	}
	if n.left == nil {
		return leaf(n.text[:at]), leaf(n.text[at:])
	}
	if at < n.left.summary.Bytes {
		left, right := split(n.left, at)
		return left, join(right, n.right)
	}
	left, right := split(n.right, at-n.left.summary.Bytes)
	return join(n.left, left), right
}

func replaceBytes(n *node, start, end int, replacement string) *node {
	total := summaryOf(n).Bytes
	start = max(0, min(start, total))
	end = max(0, min(end, total))
	left, rest := split(n, min(start, end))
	_, right := split(rest, max(start, end)-min(start, end))
	return join(join(left, build(replacement)), right)
}

func summaryOf(n *node) Summary {
	if n == nil {
		return Summary{}
	}
	return n.summary
}

// utf8DecodeRune avoids importing unicode/utf8 in three files while preserving
// the same checked decoding used to build chunks.
func utf8DecodeRune(text string) (rune, int) {
	return decodeRune(text)
}
