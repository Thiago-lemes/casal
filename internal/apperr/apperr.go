// Package apperr classifica os erros de negócio sem depender de HTTP.
// As features declaram seus erros com um Kind; a camada HTTP traduz o Kind em status.
package apperr

// Kind diz de quem é a "culpa" do erro, o que decide a resposta ao cliente.
type Kind int

const (
	Invalid      Kind = iota + 1 // dados inválidos enviados pelo cliente
	Unauthorized                 // sem login ou credenciais erradas
	Forbidden                    // logado, mas sem permissão
	NotFound                     // não existe (ou não é seu)
	Conflict                     // conflita com o estado atual
	Upstream                     // falha de um serviço externo (por exemplo, o push)
)

// Error é um erro com classificação. Use como valor sentinela: var ErrX = apperr.New(...).
// Funciona com errors.Is e com fmt.Errorf("%w: detalhe", ErrX).
type Error struct {
	Kind Kind
	Code string // opcional: código estável para o cliente traduzir; vazio usa o do Kind
	Msg  string
}

func New(kind Kind, msg string) *Error { return &Error{Kind: kind, Msg: msg} }

// NewCoded é New com um código próprio (por exemplo "note_conflict"), para o cliente
// mostrar um texto específico em vez do genérico do Kind.
func NewCoded(kind Kind, code, msg string) *Error { return &Error{Kind: kind, Code: code, Msg: msg} }

// KindCode é o código padrão de cada Kind, o mesmo que o cliente deriva do status HTTP.
func KindCode(k Kind) string {
	switch k {
	case Invalid:
		return "invalid"
	case Unauthorized:
		return "unauthorized"
	case Forbidden:
		return "forbidden"
	case NotFound:
		return "not_found"
	case Conflict:
		return "conflict"
	case Upstream:
		return "upstream"
	}
	return "unknown"
}

func (e *Error) Error() string { return e.Msg }
