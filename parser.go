package dokuwiki

import (
	"bufio"
	"io"
)

func Parse(reader io.Reader) Context {
	bufReader := bufio.NewReader(reader)
	for {
		line, err := bufReader.ReadString('\n')
		if err != nil {
			break
		}
	}
}

func Render(context Context, writer io.Writer) {
}
