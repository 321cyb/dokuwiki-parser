package dokuwiki

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
)

// Normally 0x00 won't appear in a UTF8 text, so we use it as a special marker.
var (
	startOfCodeTag   = []byte{0, 1}
	endOfCodeTag     = []byte{0, 2}
	startOfFileTag   = []byte{0, 3}
	endOfFileTag     = []byte{0, 4}
	startOfHTMLTag   = []byte{0, 5}
	endOfHTMLTag     = []byte{0, 6}
	startOfhtmlTag   = []byte{0, 7}
	endOfhtmlTag     = []byte{0, 8}
	startOfNoWikiTag = []byte{0, 9}
	endOfNoWikiTag   = []byte{0, 10}
)

const (
	noneType          = 0
	sectionHeaderType = 1
	unOrderedListType = 2
	orderedListType   = 3
	paraType          = 4
)

var (
	validSectionHeader = regexp.MustCompile(`^(=+)([^=]+)(=+)$`)
	validListItem      = regexp.MustCompile(`^((  )+)([*-]) ((?s).*)$`)
	validCodeStartTag  = regexp.MustCompile(`<code [a-zA-Z]+>$`)
	validFileStartTag  = regexp.MustCompile(`<file [a-zA-Z]+ .+>$`)
)

type wholeBlock struct {
	// blockType 1 is section header, 2 is unordered list item, 3 is ordered list item, 4 is paragraph
	blockType int

	//only meaningful when blocktype is 1
	headerLevel int

	//only meaningful when blocktype is 2 or 3
	listLevel int

	// only meaningful when blockType is 2 or 3
	forceNewList bool

	//all blockTypes need this
	rawText []byte
}

type parserStates struct {
	parseunit *ParseUnit
}

func ParseFile(filename string) *ParseUnit {
	if bytes, err := ioutil.ReadFile(filename); err == nil {
		return Parse(bytes, filepath.Base(filename))
	} else {
		return nil
	}
}

func Parse(origContent []byte, title string) *ParseUnit {
	parseunit := &ParseUnit{Title: title}
	states := parserStates{
		parseunit: parseunit,
	}

	blocks := generateLines(origContent)
	processContent(&states, blocks)

	return states.parseunit
}

// generatelines splits the raw content into lines, each line is a section or a list item or a normal paragraph.
// also removing empty lines and extra new lines.
func generateLines(origContent []byte) []wholeBlock {
	var isInCodeTag bool
	var isInFileTag bool
	var isInHTMLTag bool
	var isInhtmlTag bool
	var isInNoWikiTag bool

	blocks := make([]wholeBlock, 0)
	blockBytes := make([]byte, 0)
	lastBlockBytes := blockBytes

	// append this to make processing easier, there should be no major side effects to do this.
	origContent = append(origContent, '\n')
	physicalLines := bytes.Split(origContent, []byte{'\n'})

	for physicalLineIndex, physicalLine := range physicalLines {
		for _, b := range physicalLine {
			blockBytes = append(blockBytes, b)
			if b == '>' {
				if matchedLen := bytesEndsWithRegexp(blockBytes, validCodeStartTag); matchedLen > 0 {
					if !(isInCodeTag || isInFileTag || isInHTMLTag || isInhtmlTag || isInNoWikiTag) {
						isInCodeTag = true
						blockBytes = replaceBytesWithMarker(blockBytes, matchedLen, startOfCodeTag)
					}
				} else if bytes.HasSuffix(blockBytes, []byte{'<', '/', 'c', 'o', 'd', 'e', '>'}) {
					isInCodeTag = false
					blockBytes = replaceBytesWithMarker(blockBytes, len("</code>"), endOfCodeTag)
				} else if matchedLen := bytesEndsWithRegexp(blockBytes, validFileStartTag); matchedLen > 0 {
					if !(isInCodeTag || isInFileTag || isInHTMLTag || isInhtmlTag || isInNoWikiTag) {
						isInFileTag = true
						blockBytes = replaceBytesWithMarker(blockBytes, matchedLen, startOfFileTag)
					}
				} else if bytes.HasSuffix(blockBytes, []byte{'<', '/', 'f', 'i', 'l', 'e', '>'}) {
					isInFileTag = false
					blockBytes = replaceBytesWithMarker(blockBytes, len("</file>"), endOfFileTag)
				} else if bytes.HasSuffix(blockBytes, []byte{'<', 'h', 't', 'm', 'l', '>'}) {
					if !(isInCodeTag || isInFileTag || isInHTMLTag || isInhtmlTag || isInNoWikiTag) {
						isInhtmlTag = true
						blockBytes = replaceBytesWithMarker(blockBytes, len("<html>"), startOfhtmlTag)
					}
				} else if bytes.HasSuffix(blockBytes, []byte{'<', '/', 'h', 't', 'm', 'l', '>'}) {
					isInhtmlTag = false
					blockBytes = replaceBytesWithMarker(blockBytes, len("</html>"), endOfhtmlTag)
				} else if bytes.HasSuffix(blockBytes, []byte{'<', 'H', 'T', 'M', 'L', '>'}) {
					if !(isInCodeTag || isInFileTag || isInHTMLTag || isInhtmlTag || isInNoWikiTag) {
						isInHTMLTag = true
						blockBytes = replaceBytesWithMarker(blockBytes, len("<HTML>"), startOfHTMLTag)
					}
				} else if bytes.HasSuffix(blockBytes, []byte{'<', '/', 'H', 'T', 'M', 'L', '>'}) {
					isInHTMLTag = false
					blockBytes = replaceBytesWithMarker(blockBytes, len("</HTML>"), endOfHTMLTag)
				} else if bytes.HasSuffix(blockBytes, []byte{'<', 'n', 'o', 'w', 'i', 'k', 'i', '>'}) {
					if !(isInCodeTag || isInFileTag || isInHTMLTag || isInhtmlTag || isInNoWikiTag) {
						isInNoWikiTag = true
						blockBytes = replaceBytesWithMarker(blockBytes, len("<nowiki>"), startOfNoWikiTag)
					}
				} else if bytes.HasSuffix(blockBytes, []byte{'<', '/', 'n', 'o', 'w', 'i', 'k', 'i', '>'}) {
					isInNoWikiTag = false
					blockBytes = replaceBytesWithMarker(blockBytes, len("</nowiki>"), endOfNoWikiTag)
				}
			}
		}

		// process new line
		if isInCodeTag || isInFileTag || isInHTMLTag || isInhtmlTag || isInNoWikiTag {
			blockBytes = append(blockBytes, '\n')
		} else {
			if len(bytes.TrimSpace(blockBytes)) > 0 {
				headerLevel, headerContent := parseSectionHeader(blockBytes)
				if headerLevel > 0 {
					blocks = append(blocks, wholeBlock{
						blockType:   sectionHeaderType,
						headerLevel: headerLevel,
						rawText:     headerContent,
					})
					lastBlockBytes = blockBytes
					blockBytes = make([]byte, 0)
				} else {
					listLevel, isOrdered, itemBytes := parseListItem(blockBytes)
					if listLevel > 0 {
						block := wholeBlock{
							listLevel: listLevel,
							rawText:   itemBytes,
						}
						block.forceNewList = len(bytes.TrimSpace(lastBlockBytes)) == 0
						if isOrdered {
							block.blockType = orderedListType
						} else {
							block.blockType = unOrderedListType
						}
						blocks = append(blocks, block)
						lastBlockBytes = blockBytes
						blockBytes = make([]byte, 0)
					} else {
						nextPhysicalLine := []byte("")
						if physicalLineIndex < (len(physicalLines) - 1) {
							nextPhysicalLine = physicalLines[physicalLineIndex+1]
						}
						currentBlockStopsHere := false
						if len(bytes.TrimSpace(nextPhysicalLine)) == 0 {
							currentBlockStopsHere = true
						} else if l, _, _ := parseListItem(nextPhysicalLine); l > 0 {
							currentBlockStopsHere = true
						} else if l, _ := parseSectionHeader(nextPhysicalLine); l > 0 {
							currentBlockStopsHere = true
						} else {
							// treat new line as whitespace.
							blockBytes = append(blockBytes, ' ')
						}
						if currentBlockStopsHere {
							blocks = append(blocks, wholeBlock{
								blockType: paraType,
								rawText:   blockBytes,
							})
							lastBlockBytes = blockBytes
							blockBytes = make([]byte, 0)
						}
					}
				}
			} else {
				lastBlockBytes = blockBytes
				blockBytes = make([]byte, 0)
			}
		}
	}

	return blocks
}

// return value is the length of matched part, 0 means not match.
func bytesEndsWithRegexp(bts []byte, re *regexp.Regexp) int {
	groups := re.FindSubmatch(bts)
	if groups != nil {
		return len(groups[0])
	}
	return 0
}

func replaceBytesWithMarker(lineBytes []byte, length int, marker []byte) []byte {
	return append(lineBytes[:len(lineBytes)-length], marker...)
}

func processContent(states *parserStates, blocks []wholeBlock) {
	for _, block := range blocks {
		processLine(states, block)
	}

	//Now the skeleton is constructed, go on processing inline elements
	walkAST(states)
}

func processLine(states *parserStates, block wholeBlock) {
	if block.blockType == sectionHeaderType {
		states.parseunit.Sections = append(states.parseunit.Sections, SectionHeaderContext{
			BaseBlockContext: BaseBlockContext{BaseContext: BaseContext{*states.parseunit}},
			HeaderLevel:      block.headerLevel,
			HeaderText:       string(block.rawText),
		})
	} else if block.blockType == orderedListType || block.blockType == unOrderedListType {
		if len(states.parseunit.Sections) == 0 {
			states.parseunit.Sections = append(states.parseunit.Sections, ParaContext{
				BaseBlockContext: BaseBlockContext{BaseContext: BaseContext{*states.parseunit}},
				rawText:          string(block.rawText),
			})
		} else {
			var currentBlock = states.parseunit.Sections[len(states.parseunit.Sections)-1]
			goDeeper := true
			createTopLevelList := false

			if block.forceNewList {
				goDeeper = false
				createTopLevelList = true
			}

			for goDeeper {
				goDeeper = false
				if listBlock, isListBlock := currentBlock.(ListContext); isListBlock {
					if listBlock.Level < block.listLevel {
						if len(listBlock.InnerContexts) > 0 {
							nextLevelBlock := listBlock.InnerContexts[len(listBlock.InnerContexts)-1]
							if _, isNextLevelBlockList := nextLevelBlock.(ListContext); isNextLevelBlockList {
								currentBlock = nextLevelBlock
								goDeeper = true
							} else {
								// create a new sub list
								listBlock.InnerContexts = append(listBlock.InnerContexts, ListContext{
									BaseBlockContext: BaseBlockContext{BaseContext: BaseContext{listBlock}},
									Level:            block.listLevel,
									Ordered:          block.blockType == orderedListType,
								})
								justAddedListContext := listBlock.InnerContexts[len(listBlock.InnerContexts)-1].(ListContext)
								justAddedListContext.InnerContexts = append(justAddedListContext.InnerContexts, ParaContext{
									BaseBlockContext: BaseBlockContext{BaseContext: BaseContext{listBlock.InnerContexts[len(listBlock.InnerContexts)-1]}},
									rawText:          string(block.rawText),
								})
							}
						} else {
							// THIS SHOULD NEVER HAPPEN
							panic("PANIC!")
						}
					} else if listBlock.Level == block.listLevel {
						if listBlock.Ordered == (block.blockType == orderedListType) {
							listBlock.InnerContexts = append(listBlock.InnerContexts, ParaContext{
								BaseBlockContext: BaseBlockContext{BaseContext: BaseContext{listBlock}},
								rawText:          string(block.rawText),
							})
						} else {
							createTopLevelList = true
						}
					} else {
						createTopLevelList = true
					}
				} else {
					createTopLevelList = true
				}
			}
			if createTopLevelList {
				states.parseunit.Sections = append(states.parseunit.Sections, ListContext{
					BaseBlockContext: BaseBlockContext{BaseContext: BaseContext{*states.parseunit}},
					Level:            block.listLevel,
					Ordered:          block.blockType == orderedListType,
				})
				justAddedListContext := states.parseunit.Sections[len(states.parseunit.Sections)-1].(ListContext)
				justAddedListContext.InnerContexts = append(justAddedListContext.InnerContexts, ParaContext{
					BaseBlockContext: BaseBlockContext{BaseContext: BaseContext{states.parseunit.Sections[len(states.parseunit.Sections)-1]}},
					rawText:          string(block.rawText),
				})
			}
		}
	} else {
		states.parseunit.Sections = append(states.parseunit.Sections, ParaContext{
			BaseBlockContext: BaseBlockContext{BaseContext: BaseContext{*states.parseunit}},
			rawText:          string(block.rawText),
		})
	}
}

func walkAST(states *parserStates) {
}

/*
func processCharacter(states *parserStates, character byte) {
	if states.isInTag() {
		if character == '<' {
			processLeftBracket(states)
		} else {
			states.currentContent = append(states.currentContent, character)
		}
	} else {
		switch character {
		case '\n':
			processNewLine(states)
		case '\'':
		case '_':
		case '/':
		case '=':
		case '*':
		case '-':
		case '<':
			processLeftBracket(states)
		case '[':
		case ']':
		case '{':
		case '}':

		default:
			if character == '\n' || character == '\t' || character == ' ' {
			}
		}
	}

}

// advanceIfMatch consumes the buffered reader if the text matches.
func advanceIfMatch(states *parserStates, text string, caseSensitive bool) bool {
	nbytes := len(text)
	ahead := string(states.origContent[states.offset : states.offset+nbytes])
	matches := false
	if err == nil {
		if caseSensitive {
			if ahead == text {
				matches = true
			}
		} else {
			if strings.ToLower(ahead) == strings.ToLower(text) {
				matches = true
			}
		}
	}
	if matches {
		states.offset += nbytes
	}
	return matches
}

// processnewline is only called in Non Tag state.
func processNewLine(states *parserStates) {
	nextIsNewLine := states.origContent[states.offset+1] == '\n'

	for {
		c := states.origContent[states.offset]
		if c != '\n' && c != '\t' && c != ' ' {
			break
		}
		states.offset++
	}

	nextLineBytes := peekNextLine(states)

	switch states.currentContext.(type) {
	case *SectionContext:
		states.currentContext.HeaderText = string(states.currentContent)
	case *ListContext:
	case *ParaContext:
		if !nextIsNewLine {

			return
		}

	default:
		fmt.Fprintln("processNewLine error: wrong currentContext")
		panic()
	}
}

// peek the next line starts from current offset
func peekNextLine(states *parserStates) []byte {
	var nextLineBytes []byte
	newLineOffset := states.offset
	for {
		if newLineOffset >= len(states.origContent) || states.origContent[newLineOffset] != '\n' {
			break
		}
		nextLineBytes = append(nextLineBytes, states.origContent[newLineOffset])
		newLineOffset++
	}
	return nextLineBytes
}
*/

// returns the section header level, 0 means not a header.
func parseSectionHeader(line []byte) (int, []byte) {
	lineString := strings.TrimRight(string(line), "\t ")
	groups := validSectionHeader.FindStringSubmatch(lineString)
	if groups != nil && (len(groups[1]) == len(groups[3])) {
		return len(groups[1]), []byte(strings.TrimSpace(groups[2]))
	}
	return 0, nil
}

// returns the list item level, 0 means not a list
func parseListItem(line []byte) (int, bool, []byte) {
	lineString := string(line)
	groups := validListItem.FindStringSubmatch(lineString)
	if groups != nil && len(groups[1]) >= 2 {
		starOrDash := groups[3]
		return len(groups[1]), starOrDash == "-", []byte(strings.TrimSpace(groups[4]))
	}
	return 0, false, nil
}

func Render(unit *ParseUnit, writer io.Writer) {
}
