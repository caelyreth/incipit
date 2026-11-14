// Package search finds pages in a notebook: an index of every page kept by
// Bleve (references/search-engine.md), kept in step by the Indexer, run by
// queries the query language reads (internal/query). It has no persistence
// or UI dependency beyond the index's own folder.
//
// One analyzer reads every text the package sees (query.Tokens): the
// index's fields, a query's words and a page's text a snippet comes from.
// So the index, a highlight and a sheet's find never disagree on what
// matches.
//
// Bleve stays inside this package: nothing it returns names a Bleve type,
// and an index, on disk or in memory, opens only on scorch.
package search
