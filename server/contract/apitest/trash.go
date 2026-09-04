package apitest

import "testing"

// EmptyTrash submits the empty-trash chain and asserts the status the endpoint
// answers the submission with.
func EmptyTrash(t *testing.T, env *Env, wantStatus int) TaskChainSubmitResponse {
	t.Helper()
	return SubmitTaskChain(t, env, EmptyTrashURL(), nil, wantStatus)
}
