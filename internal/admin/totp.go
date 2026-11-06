package admin

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" // #nosec: o TOTP (RFC 6238) é definido sobre HMAC-SHA1; é o que os aplicativos autenticadores esperam
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	totpPeriod = 30 // segundos por passo
	totpDigits = 6
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewSecret gera um segredo TOTP de 160 bits, em base32 (o formato que os aplicativos autenticadores leem).
func NewSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return b32.EncodeToString(b), nil
}

// OtpauthURI é o endereço que vira QR code no aplicativo autenticador.
func OtpauthURI(email, secret string) string {
	q := url.Values{"secret": {secret}, "issuer": {"Zoog admin"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + url.PathEscape("Zoog admin:"+email) + "?" + q.Encode()
}

// totpCode calcula o código de um passo (RFC 4226/6238).
func totpCode(secret string, step int64) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil {
		return "", fmt.Errorf("segredo TOTP inválido: %w", err)
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	bin := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	mod := uint32(1)
	for i := 0; i < totpDigits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", totpDigits, bin%mod), nil
}

// verifyTOTP confere o código nos passos vizinhos (relógios levemente fora de hora) e devolve o passo que bateu.
// Só vale um passo maior que lastStep: o mesmo código nunca entra duas vezes, nem em outro passo mais antigo.
func verifyTOTP(secret, code string, now time.Time, lastStep int64) (step int64, ok bool) {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return 0, false
	}
	cur := now.Unix() / totpPeriod
	for _, s := range []int64{cur - 1, cur, cur + 1} {
		if s <= lastStep {
			continue
		}
		want, err := totpCode(secret, s)
		if err != nil {
			return 0, false
		}
		if hmac.Equal([]byte(want), []byte(code)) {
			return s, true
		}
	}
	return 0, false
}

// Code calcula o código TOTP de um instante. Serve aos testes e a quem precisa conferir o segredo; o login usa o
// verifyTOTP, que também barra o reaproveitamento.
func Code(secret string, at time.Time) (string, error) {
	return totpCode(secret, at.Unix()/totpPeriod)
}
