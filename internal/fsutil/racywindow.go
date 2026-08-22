package fsutil

import "time"

// RacyWindow is how recently a file or directory may have been modified before
// its stat can be trusted enough to remember across runs.
//
// Timestamps are coarse: ext3 and HFS+ store whole seconds, and a FAT-backed
// share reached over SMB stores two. Something modified again inside the same
// tick keeps the mtime that was just recorded, and a cache that remembered that
// stat would go on believing it - a directory that looks unchanged forever, or
// a fingerprint served for content that has already been replaced. Trusting a
// stat only once its mtime is older than this window closes that race: a
// modification that could still share the recorded timestamp is left out of the
// cache, so the next run reads it normally. This is the "racily clean" rule Git
// applies to its index.
//
// Both the directory scan cache and the fingerprint index apply this rule, and
// the value has to match between them, so it lives here rather than being
// written twice.
const RacyWindow = 2 * time.Second
