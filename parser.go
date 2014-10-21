package dokuwiki

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode"
)

type parser struct {
	currentSectionLevel int
	currentListLevel    int
}

func ParseFile(filename string) *ParseUnit {
	reader, err := os.Open(filename)
	defer reader.Close()
	if err != nil {
		return Parse(reader, filepath.Base(filename))
	} else {
		return nil
	}
}

func lexer(reader io.Reader) {
	bufReader := bufio.NewReader(reader)
	var character, lastCharacter rune
	var err error
	for {
		character, _, err = bufReader.ReadRune()
		if err != nil {
			fmt.FPrintf(os.Stderr, "bufReader.ReadRune error %s\n", err)
			break
		}

		switch character {
		case '\n':
		case '=':
		case '-':
		case '_':
		case '*':
		case '/':
		case '<':
		case '>':
		case '[':
		case ']':

		default:
			if unicode.IsSpace(character) {
			}
		}

		lastCharacter = character
	}
}

func Parse(reader io.Reader, title string) *ParseUnit {
	parseunit := &ParseUnit{Title: title, InnerContexts: make([]*Context, 1)}
	ss := parser{}
	for {
	}
	return parseunit
}

func Render(unit *ParseUnit, writer io.Writer) {
}
