package passwords

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := Hash("hunter2hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if !Verify("hunter2hunter2", hash) {
		t.Error("correct password rejected")
	}
	if Verify("hunter2hunter3", hash) {
		t.Error("wrong password accepted")
	}
}

func TestVerifyMalformed(t *testing.T) {
	for _, stored := range []string{"", "not base64!", "c2hvcnQ="} {
		if Verify("x", stored) {
			t.Errorf("accepted malformed hash %q", stored)
		}
	}
}
