package apitest

import "testing"

func EmptyTrash(t *testing.T, env *Env, wantStatus int) TaskChainSubmitResponse {
	t.Helper()
	return SubmitTaskChain(t, env, EmptyTrashURL(), nil, wantStatus)
}
