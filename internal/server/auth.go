package server

import (
	"github.com/instructkr/smartclaw/internal/serverauth"
)

type AuthManager = serverauth.AuthManager

type Session = serverauth.Session

var NewAuthManager = serverauth.NewAuthManager

const SessionDuration = serverauth.SessionDuration
