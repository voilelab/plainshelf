package server

import (
	"errors"
	"net/http"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
)

type taskSubmitter struct {
	*apiCore

	pool *taskutil.Pool
}

type taskChainSubmitResponse struct {
	TaskChainID string `json:"taskchain_id"`
}

func (t *taskSubmitter) submitTaskChain(w http.ResponseWriter, r *http.Request, chain *taskutil.TaskChain, fallback string) {

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
