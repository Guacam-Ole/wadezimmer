package routes

// Keep the result shared with waiting requests separate from work performed
// for the current request. Waiting for an error does not permit another fetch.
type knownInboxActorReloadError struct {
	err          error
	fetched      bool
	claimRefused bool
}

func (e *knownInboxActorReloadError) Error() string { return e.err.Error() }

func (e *knownInboxActorReloadError) Unwrap() error { return e.err }

// The callback already logged the diagnostic details. Callers use a neutral
// error so a shared refresh failure cannot disclose them in an HTTP response.
type knownInboxActorReloadPanicError struct{}

func (e *knownInboxActorReloadPanicError) Error() string { return "actor refresh did not complete" }
