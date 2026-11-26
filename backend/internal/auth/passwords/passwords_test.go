package passwords

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestHashThenVerify(t *testing.T) {
	hash, err := Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}

	if !Verify("correct horse battery", hash) {
		t.Error("the right password was rejected")
	}
}

func TestVerifyRejectsTheWrongPassword(t *testing.T) {
	hash, err := Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}

	for name, password := range map[string]string{
		"a different password":  "wrong horse battery",
		"the empty password":    "",
		"the right one, padded": "correct horse battery ",
		"the right one, cased":  "Correct horse battery",
	} {
		if Verify(password, hash) {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestAnEmptyPasswordCanBeHashedAndOnlyMatchesItself(t *testing.T) {
	hash, err := Hash("")
	if err != nil {
		t.Fatal(err)
	}

	if !Verify("", hash) || Verify("x", hash) {
		t.Error("the empty password must verify against its own hash and nothing else")
	}
}

func TestHashesOfTheSamePasswordDiffer(t *testing.T) {
	first, err1 := Hash("same password")
	second, err2 := Hash("same password")
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}

	if first == second {
		t.Error("two hashes of one password are identical, so the salt is not random")
	}
	if !Verify("same password", first) || !Verify("same password", second) {
		t.Error("both hashes must verify")
	}
}

func TestHashLayoutIsSaltThenKey(t *testing.T) {
	hash, err := Hash("layout")
	if err != nil {
		t.Fatal(err)
	}

	raw, err := base64.StdEncoding.DecodeString(hash)
	if err != nil {
		t.Fatalf("a hash is not standard base64: %v", err)
	}
	if want := saltLen + keyLen; len(raw) != want {
		t.Errorf("hash decodes to %d bytes, want %d (salt %d + key %d)", len(raw), want, saltLen, keyLen)
	}
	if strings.Contains(hash, "layout") {
		t.Error("the hash contains the password")
	}
}

func TestVerifyNeverMatchesAMalformedHash(t *testing.T) {
	good, err := Hash("password")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(good)
	enc := base64.StdEncoding.EncodeToString

	tests := map[string]string{
		"empty":                     "",
		"not base64":                "!!! not base64 !!!",
		"the plain password":        "password",
		"a salt and no key":         enc(raw[:saltLen]),
		"less than a salt":          enc(raw[:saltLen-1]),
		"the right salt, wrong key": enc(append(append([]byte{}, raw[:saltLen]...), make([]byte, keyLen)...)),
		"a key of the wrong value":  enc(append(append([]byte{}, raw[:saltLen]...), []byte(strings.Repeat("x", keyLen))...)),
	}
	for name, hash := range tests {
		t.Run(name, func(t *testing.T) {
			if Verify("password", hash) {
				t.Error("verified against a malformed hash")
			}
		})
	}
}
