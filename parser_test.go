package dokuwiki

import (
	_ "fmt"
	"testing"
)

func TestSimpleParseSectionHeader(t *testing.T) {
	level, content := parseSectionHeader([]byte("=== abc === "))
	if level != 3 {
		t.Fail()
	}
	if string(content) != "abc" {
		t.Fail()
	}
}

func TestSimpleListItem(t *testing.T) {
	level, isOrdered, content := parseListItem([]byte("  - abc "))
	if level != 2 {
		t.Fail()
	}
	if !isOrdered {
		t.Fail()
	}
	if string(content) != "abc" {
		t.Fail()
	}
}

func TestReplaceBytes(t *testing.T) {
	ret := replaceBytesWithMarker([]byte("ab<code>"), len("<code>"), []byte{0x00, 0x01})
	if len(ret) != 4 || ret[0] != 'a' || ret[1] != 'b' || ret[2] != 0x00 || ret[3] != 0x01 {
		t.Fail()
	}
}

func TestA(t *testing.T) {
	ParseFile("/home/turing/how_to_write_a_compiler.txt")
}
