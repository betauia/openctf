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

// Hash produced by _hash in backend/app/api/auth.py, so migrated users can still log in.
func TestVerifyPythonHash(t *testing.T) {
	const pyHash = "D5xT8xhRpsAsQdFeDV4LTI5Cy9kGtjDDAjmdHDAHVMeCtuyaWAy16FIofy2MqHC5"
	if !Verify("correct horse battery", pyHash) {
		t.Error("python hash rejected")
	}
	if Verify("wrong", pyHash) {
		t.Error("wrong password accepted for python hash")
	}
}

func TestVerifyMalformed(t *testing.T) {
	for _, stored := range []string{"", "not base64!", "c2hvcnQ="} {
		if Verify("x", stored) {
			t.Errorf("accepted malformed hash %q", stored)
		}
	}
}
