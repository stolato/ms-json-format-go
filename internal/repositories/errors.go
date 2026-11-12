package repositories

// PublicError é um erro cuja mensagem pode ser mostrada ao cliente. Qualquer
// outro erro (ex.: do Mongo) é registrado no log e respondido de forma genérica.
type PublicError struct{ msg string }

func (e *PublicError) Error() string { return e.msg }

func newPublicError(msg string) error { return &PublicError{msg: msg} }
