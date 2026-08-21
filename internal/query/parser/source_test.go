package parser

import "testing"

func TestRememberSourceRoundTrip(t *testing.T) {
	cmd, _, err := Parse[string, float32]("remember@2 'fact' source:'Tool Call / Session 17'")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	remember := cmd.(*RememberCommandNode[float32])
	if got, want := remember.Source(), "Tool Call / Session 17"; got != want {
		t.Fatalf("Source() = %q, want %q", got, want)
	}
	if got, want := remember.String(), "remember@2 'fact' source:'Tool Call / Session 17'"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestRememberRejectsDuplicateSource(t *testing.T) {
	_, _, err := Parse[string, float32]("remember 'fact' source:'one' source:'two'")
	if err == nil {
		t.Fatal("duplicate source parsed successfully")
	}
}

func TestRecallDoesNotAcceptSourceClause(t *testing.T) {
	_, _, err := Parse[string, float32]("recall fact source:'one'")
	if err == nil {
		t.Fatal("source clause unexpectedly accepted on recall")
	}
}
