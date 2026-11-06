package main

type nativeUI interface {
	openDirectory(title string) (string, error)
	showError(title, message string) error
	reloadApp()
	focus()
}
