package apperr

import "testing"

func TestExitCode(t *testing.T) {
	if ExitCode(KindConnection) != 2 {
		t.Fatal()
	}
	if ExitCode(KindConfig) != 3 {
		t.Fatal()
	}
	if ExitCode(KindReadOnly) != 4 {
		t.Fatal()
	}
	if ExitCode(KindAPI) != 1 {
		t.Fatal()
	}
}

func TestAsKind(t *testing.T) {
	err := Wrap(KindSession, "boom", nil)
	k, ok := AsKind(err)
	if !ok || k != KindSession {
		t.Fatalf("%v %v", k, ok)
	}
}
