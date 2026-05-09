//go:build android

package txtlibgui

// Currently, fyne's URI-based FS doesn't support write operations on Android,
// so we have to open the library in read-only mode.
// This means that users won't be able to add/remove books or edit book metadata when using the app on Android. We can revisit this in the future if fyne adds support for writable URI FS on Android.
var libReadonly = true
