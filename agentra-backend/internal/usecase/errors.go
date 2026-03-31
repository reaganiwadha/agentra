package usecase

import "github.com/joomcode/errorx"

var ns = errorx.NewNamespace("agentra")

var (
	ErrNotFound           = errorx.NewType(ns, "not_found")
	ErrBadRequest         = errorx.NewType(ns, "bad_request")
	ErrInvalidCredentials = errorx.NewType(ns, "invalid_credentials")
	ErrInvalidSetupToken  = errorx.NewType(ns, "invalid_setup_token")
	ErrSetupDone          = errorx.NewType(ns, "setup_done")
	ErrConflict           = errorx.NewType(ns, "conflict")
	ErrForbidden          = errorx.NewType(ns, "forbidden")
)
