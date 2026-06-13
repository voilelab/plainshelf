package server

import (
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// RegisterShelf adds a shelf to this in-process app instance without exposing
// any REST mutation API. Desktop mode uses this to make newly persisted local
// shelves visible to existing shelf-scoped handlers immediately.
func (app *App) RegisterShelf(conf shelf.ShelfConfWithID) error {
	if app == nil || app.shelfManager == nil {
		return util.NewError("app shelf manager is not initialized")
	}
	if err := app.shelfManager.AddShelf(conf); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

// UnregisterShelf removes a shelf from this in-process app instance. It is used
// by desktop mode to roll back runtime registration if local config persistence
// fails.
func (app *App) UnregisterShelf(id string) error {
	if app == nil || app.shelfManager == nil {
		return util.NewError("app shelf manager is not initialized")
	}
	if err := app.shelfManager.RemoveShelf(id); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
