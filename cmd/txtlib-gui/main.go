package main

import (
	"os"

	txtlibgui "github.com/voilelab/plainshelf/txtlib-gui"
)

func main() {
	os.Setenv("FYNE_SCALE", "1.6")

	app := txtlibgui.NewTxtlibApp()
	app.Run()
}
