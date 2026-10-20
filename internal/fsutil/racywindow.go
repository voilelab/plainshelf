package fsutil

import "time"

// RacyWindow is how recently a file or directory may have been modified before
// its stat can be trusted enough to remember across runs. ext3 and HFS+ store
// whole seconds and FAT over SMB stores two, so a change inside the same tick
// keeps the recorded mtime. A cache holding that stat would trust it forever: a
// directory that never looks changed, or a fingerprint for replaced content.
// Stats younger than this window are left out of the cache and read normally
// next run, Git's "racily clean" rule. The scan cache and the fingerprint index
// must use the same value, so it is defined once here.
const RacyWindow = 2 * time.Second
