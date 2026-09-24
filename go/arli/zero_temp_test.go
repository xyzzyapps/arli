package main

import (
	"strings"
	"testing"
)

func TestZeroTempDoesNotHallucinate(t *testing.T) {
	text, err := zeroTempReport()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"Ancestor(Alice, Charlie)  1",
		"Parent(Alice, Charlie)    0",
		"Ancestor(Alice, Dana)     0",
		"T = 0   Ancestor(Alice, Charlie) 1    Ancestor(Alice, Dana) 0",
	} {
		if !strings.Contains(text, line) {
			t.Fatalf("report missing %q\n%s", line, text)
		}
	}
	hot := lineAfter(text, "T = 1")
	if !strings.Contains(hot, "Dana) 0.") {
		t.Fatalf("high T did not let Dana borrow: %s", hot)
	}
}

func lineAfter(s, prefix string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, prefix) {
			return line
		}
	}
	return ""
}
