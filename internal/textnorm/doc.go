// Package textnorm turns text into the canonical form similarity detection
// compares, and names the hash that form is reduced with.
//
// Two books hold the same text far more often than the same bytes: reflowed
// lines, corner brackets for curly quotes, ideographic indents. Normalize strips
// that layer, and Hash64Version names the hash internal/sketch applies to the
// result. Both are versioned and belong in the algo block of any cache built on
// them, since a fingerprint is comparable only to one made by the same rules,
// including on another machine sharing the shelf.
package textnorm
