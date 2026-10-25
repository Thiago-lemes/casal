package auth

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("senha-forte-123")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("senha-forte-123", hash) {
		t.Error("a senha correta deveria passar")
	}
	if VerifyPassword("outra-senha", hash) {
		t.Error("a senha errada não deveria passar")
	}
	if VerifyPassword("senha-forte-123", "lixo") {
		t.Error("um hash malformado não deveria passar")
	}

	otherHash, _ := HashPassword("senha-forte-123")
	if hash == otherHash {
		t.Error("o mesmo texto deve gerar hashes diferentes (salt aleatório)")
	}
}
