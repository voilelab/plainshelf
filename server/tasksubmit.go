package server

import (
	"errors"
	"net/http"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
)

// taskSubmitter is the shared half of every endpoint that starts background
// work: it resolves the shelf like any other handler, then hands the chain to
// the pool and answers for it.
type taskSubmitter struct {
	*apiCore

	pool *taskutil.Pool
}

type taskChainSubmitResponse struct {
	TaskChainID string `json:"taskchain_id"`
}

// submitTaskChain answers 202 with the new chain's ID, or 409 with the ID of
// the chain already in flight so the client can attach to it instead.
func (t *taskSubmitter) submitTaskChain(w http.ResponseWriter, r *http.Request, chain *taskutil.TaskChain, fallback string) {
	// The chain inherits the ID of the request that queued it rather than
	// minting one of its own: the 202 this answers with already carries that ID
	// in X-Request-Id, so the number the user has is the number the chain's
	// later failures are logged under.
	chain.RequestID = logutil.RequestIDFrom(r.Context())

	submitted, err := t.pool.Submit(chain)
	switch {
	case errors.Is(err, taskutil.ErrTaskChainRunning):
		t.writeJSON(w, http.StatusConflict, taskChainSubmitResponse{TaskChainID: submitted.ID})
	case err != nil:
		t.writeErr(w, r, err, fallback)
	default:
		t.writeJSON(w, http.StatusAccepted, taskChainSubmitResponse{TaskChainID: submitted.ID})
	}
}
