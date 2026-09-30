// Package textnorm turns text into the canonical form similarity detection
// compares, and names the hash that form is reduced with.
//
// Two books hold "the same text" far more often than they hold the same bytes:
// a reflowed line width, curly quotes traded for corner brackets, an ideographic
// space indenting every paragraph. Normalize strips that layer away so the
// remaining characters are the content itself, and Hash64Version names the hash
// internal/sketch reduces a piece of that content to a number with.
//
// Both operations are versioned, and both versions belong in the algo block of
// any cache built on top of them. A fingerprint is only ever comparable to
// another fingerprint produced by the same rules — including one produced on a
// different machine, which is the case PlainShelf has to get right, because a
// shelf is routinely shared between them.
package textnorm
