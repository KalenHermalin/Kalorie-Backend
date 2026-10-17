package handlers

import (
	"errors"
	"main/internal/apperrors"
	"main/internal/loggerx"
	"net/http"
)

type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

func Handle(fn HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := fn(w, r)
		if err == nil {
			return // Done we already wrote a succesful response
		}
		var kerr *apperrors.KError
		if !errors.As(err, &kerr) { // Search error chain and fills kerr with a KError found in the chain
			kerr = apperrors.New(apperrors.Internal, err)
		}
		apperrors.Write(w, kerr)
		lgx := loggerx.Log(r.Context())
		// Log err because KError implements Error so if err is a Kerror we get caused log, else we log a normal error
		if kerr.Kind.Status >= 500 {
			lgx.Error("Request failed", "code", kerr.Kind.Code, "err", err)
		} else {
			lgx.Warn("Request warning", "code", kerr.Kind.Code, "err", err)
		}
	}
}
