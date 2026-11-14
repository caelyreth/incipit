// Package search finds pages in a notebook: an index of every page kept by
// Bleve (references/search-engine.md), a query language of words, phrases
// and $ keys, and the matcher that places what a query matches in a text.
// It has no persistence or UI dependency beyond the index's own folder.
//
// One analyzer reads every text the package sees (Tokens): the index's
// fields, a query's words and a page's text a snippet comes from. So the
// index, a highlight and a sheet's find never disagree on what matches.
//
// Bleve stays inside this package: nothing it returns names a Bleve type,
// and an index, on disk or in memory, opens only on scorch.
package search
