package main

type dialogKind int

const (
	errorDialog dialogKind = iota
	warningDialog
)

type fileFilter struct {
	label   string
	pattern string
}

type nativeUI interface {
	openFiles(title string, filters []fileFilter) ([]string, error)
	openDirectory(title string) (string, error)
	saveFile(title, defaultName string, filters []fileFilter) (string, error)
	showMessage(kind dialogKind, title, message string) error
	openURL(rawURL string)
	execJS(script string)
	quit()
	copyText(text string)
	showWindow()
	emit(name string, data any)
	setTrayMenu(items []menuItem)
}
