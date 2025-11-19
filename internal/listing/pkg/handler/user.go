package handler

import (
	"net/http"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
)

func (h *Handler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	log.Info().Msg("user successfully registered")
	res := httpHelper.Response{
		Success: true,
		Message: "user created successfully",
	}

	httpHelper.SendJSON(w, res, http.StatusCreated)
}
