package object_store

import "errors"

// ErrUnauthorized signals an AWS auth failure (bad key, wrong
// signature, expired token). The runtime contract does not define an
// ErrUnauthorized today; if it gains one later, mapErr should prefer
// the contract value. See §3.5 of
// docs/proposals/hostpoint-complete-roadmap-2026-05-18.md.
var ErrUnauthorized = errors.New("object-store: unauthorized")
