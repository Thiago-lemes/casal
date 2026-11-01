package httpx

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"

	"casal/internal/apperr"
)

// Fail traduz um erro de negócio na resposta HTTP. É o único lugar que decide status.
//
//   - erros do cliente (Invalid, Unauthorized, Forbidden, NotFound, Conflict) mostram a mensagem;
//   - falhas de serviço externo viram 502 e qualquer outro erro vira 500, sem expor detalhes,
//     e são registrados no log com o ID da requisição.
func Fail(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		switch ae.Kind {
		case apperr.Invalid:
			Error(w, http.StatusBadRequest, err.Error())
		case apperr.Unauthorized:
			Error(w, http.StatusUnauthorized, err.Error())
		case apperr.Forbidden:
			Error(w, http.StatusForbidden, err.Error())
		case apperr.NotFound:
			Error(w, http.StatusNotFound, err.Error())
		case apperr.Conflict:
			Error(w, http.StatusConflict, err.Error())
		case apperr.Upstream:
			logFailure(log, r, err, slog.LevelWarn)
			Error(w, http.StatusBadGateway, ae.Msg)
		default:
			logFailure(log, r, err, slog.LevelError)
			Error(w, http.StatusInternalServerError, "erro interno")
		}
		return
	}
	logFailure(log, r, err, slog.LevelError)
	Error(w, http.StatusInternalServerError, "erro interno")
}

func logFailure(log *slog.Logger, r *http.Request, err error, level slog.Level) {
	log.Log(r.Context(), level, "falha ao atender requisição",
		"err", err, "method", r.Method, "path", r.URL.Path, "request_id", RequestIDFrom(r.Context()))
}

// Bind lê o corpo JSON em dst. Se for inválido, já responde 400 e devolve false.
func Bind(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := Decode(w, r, dst); err != nil {
		Error(w, http.StatusBadRequest, "JSON inválido")
		return false
	}
	return true
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsUUID evita mandar ao banco um id malformado, que viraria erro 500.
func IsUUID(s string) bool { return uuidRe.MatchString(s) }
