package main

type nativeUI interface {
	newWindow(prepare func(win uint)) uint
	currentWindow() uint
	openDirectory(win uint, title string) (string, error)
	showError(win uint, title, message string) error
	reload(win uint)
	focus(win uint)
}
