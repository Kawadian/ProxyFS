package api

import (
	"net/http"

	"github.com/lxcfh/lxcfh/internal/httputil"
)

const ProblemContentType = httputil.ProblemContentType

type Problem = httputil.Problem

func WriteProblem(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	httputil.WriteProblem(w, r, status, title, detail)
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	httputil.WriteJSON(w, status, v)
}

func DecodeJSON(r *http.Request, v any) error {
	return httputil.DecodeJSON(r, v)
}
