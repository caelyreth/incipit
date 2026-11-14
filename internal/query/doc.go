// Package query is the language a search is written in, and how a text
// is read for it: Parse reads a query of words, phrases and $ keys, never
// failing; Tokens reads a text into the terms a query matches, one
// analyzer for every text; a Matcher places what a query matches in a
// text, and Complete offers what may stand at a caret in a query.
//
// It is pure: no index, no storage, no UI. The notebook's index
// (app/search) and a sheet's find (editor/policies/find) both read with
// it, so they never disagree on what matches.
package query
