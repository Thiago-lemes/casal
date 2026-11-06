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
		// o código é o do erro (ou do Kind); o cliente traduz por ele e não depende da frase
		code := ae.Code
		if code == "" {
			code = apperr.KindCode(ae.Kind)
		}
		switch ae.Kind {
		case apperr.Invalid:
			ErrorCode(w, http.StatusBadRequest, code, err.Error())
		case apperr.Unauthorized:
			ErrorCode(w, http.StatusUnauthorized, code, err.Error())
		case apperr.Forbidden:
			ErrorCode(w, http.StatusForbidden, code, err.Error())
		case apperr.NotFound:
			ErrorCode(w, http.StatusNotFound, code, err.Error())
		case apperr.Conflict:
			ErrorCode(w, http.StatusConflict, code, err.Error())
		case apperr.Upstream:
			logFailure(log, r, err, slog.LevelWarn)
			ErrorCode(w, http.StatusBadGateway, code, ae.Msg)
		default:
			logFailure(log, r, err, slog.LevelError)
			ErrorCode(w, http.StatusInternalServerError, "internal", "erro interno")
		}
		return
	}
	logFailure(log, r, err, slog.LevelError)
	ErrorCode(w, http.StatusInternalServerError, "internal", "erro interno")
}

func logFailure(log *slog.Logger, r *http.Request, err error, level slog.Level) {
	log.Log(r.Context(), level, "falha ao atender requisição",
		"err", err, "method", r.Method, "path", r.URL.Path, "request_id", RequestIDFrom(r.Context()))
	if level >= slog.LevelError {
		reportServerError(r, err, nil)
	}
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
