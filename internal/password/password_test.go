package password

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := Hash("senha-forte-123")
	if err != nil {
		t.Fatal(err)
	}
	if !Verify("senha-forte-123", hash) {
		t.Error("a senha correta deveria passar")
	}
	if Verify("outra-senha", hash) {
		t.Error("a senha errada não deveria passar")
	}
	if Verify("senha-forte-123", "lixo") {
		t.Error("um hash malformado não deveria passar")
	}

	otherHash, _ := Hash("senha-forte-123")
	if hash == otherHash {
		t.Error("o mesmo texto deve gerar hashes diferentes (salt aleatório)")
	}
}
