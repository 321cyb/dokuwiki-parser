package dokuwiki

import (
	"bytes"
	_ "fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
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
	validMedia         = regexp.MustCompile(`^(.*?)(\?\d+(x\d+)?)?$`)
	validURL           = regexp.MustCompile(`(https?|ftp)://[^\s/$.?#].[^\s]*`)
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

//TODO: http in ordinary text,
//TODO: add offset
func parsePara(c *ParaContext) {
	rawTextBytes := []byte(c.rawText)

	var currentEffect uint32 = 0
	effectBytes := make([]byte, 0)
	offset := 0

	for offset < len(rawTextBytes) {
		ch := rawTextBytes[offset]
		switch ch {
		case 0x00:
			//This is the beginning or end of a tag.
			nextByte := rawTextBytes[offset+1]
			if nextByte == 1 || nextByte == 3 || nextByte == 5 || nextByte == 7 || nextByte == 9 {
				endCurrentEffect(c, &effectBytes, currentEffect)
				currentEffect = 0
			}

			switch nextByte {
			case 1:
				if i := bytes.Index(rawTextBytes[offset:], endOfCodeTag); i != -1 {
					c.InnerContexts = append(c.InnerContexts, CodeFileContext{
						BaseInlineContext: BaseInlineContext{BaseContext{parent: *c}},
						Text:              string(rawTextBytes[offset+2 : offset+i]),
					})
				} else {
					panic("no endOfCode tag found!")
				}
			case 3:
				if i := bytes.Index(rawTextBytes[offset:], endOfFileTag); i != -1 {
					c.InnerContexts = append(c.InnerContexts, CodeFileContext{
						BaseInlineContext: BaseInlineContext{BaseContext{parent: *c}},
						Text:              string(rawTextBytes[offset+2 : offset+i]),
					})
				} else {
					panic("no endOfFile tag found!")
				}
			case 5:
				if i := bytes.Index(rawTextBytes[offset:], endOfHTMLTag); i != -1 {
					c.InnerContexts = append(c.InnerContexts, HTMLContext{
						BaseInlineContext: BaseInlineContext{BaseContext{parent: *c}},
						Text:              string(rawTextBytes[offset+2 : offset+i]),
					})
				} else {
					panic("no endOfHTML tag found!")
				}
			case 7:
				if i := bytes.Index(rawTextBytes[offset:], endOfhtmlTag); i != -1 {
					c.InnerContexts = append(c.InnerContexts, HTMLContext{
						BaseInlineContext: BaseInlineContext{BaseContext{parent: *c}},
						Text:              string(rawTextBytes[offset+2 : offset+i]),
					})
				} else {
					panic("no endOfhtml tag found!")
				}
			case 9:
				if i := bytes.Index(rawTextBytes[offset:], endOfNoWikiTag); i != -1 {
					c.InnerContexts = append(c.InnerContexts, NoWikiContext{
						BaseInlineContext: BaseInlineContext{BaseContext{parent: *c}},
						Text:              string(rawTextBytes[offset+2 : offset+i]),
					})
				} else {
					panic("no endOfNoWiki tag found!")
				}
			}
		case '`':
			if rawTextBytes[offset+1] == '`' {
				endCurrentEffect(c, &effectBytes, currentEffect)
				if (currentEffect & TextEffectMonoSpace) > 0 {
					currentEffect ^= TextEffectMonoSpace
				} else {
					currentEffect |= TextEffectMonoSpace
				}
			} else {
				effectBytes = append(effectBytes, ch)
			}
		case '_':
			if rawTextBytes[offset+1] == '_' {
				endCurrentEffect(c, &effectBytes, currentEffect)
				if (currentEffect & TextEffectUnderline) > 0 {
					currentEffect ^= TextEffectUnderline
				} else {
					currentEffect |= TextEffectUnderline
				}
			} else {
				effectBytes = append(effectBytes, ch)
			}
		case '/':
			if rawTextBytes[offset+1] == '/' {
				endCurrentEffect(c, &effectBytes, currentEffect)
				if (currentEffect & TextEffectItalic) > 0 {
					currentEffect ^= TextEffectItalic
				} else {
					currentEffect |= TextEffectItalic
				}
			} else {
				effectBytes = append(effectBytes, ch)
			}
		case '*':
			if rawTextBytes[offset+1] == '*' {
				endCurrentEffect(c, &effectBytes, currentEffect)
				if (currentEffect & TextEffectBold) > 0 {
					currentEffect ^= TextEffectBold
				} else {
					currentEffect |= TextEffectBold
				}
			} else {
				effectBytes = append(effectBytes, ch)
			}
		case '[':
			if rawTextBytes[offset+1] == '[' {
				// start of a link.
				if i := bytes.Index(rawTextBytes[offset:], []byte{']', ']'}); i != -1 {
					endCurrentEffect(c, &effectBytes, currentEffect)
					currentEffect = 0
					parseLink(c, rawTextBytes[offset+2:offset+i])
					offset += (i + 2)
				}
			}
		case '{':
			if rawTextBytes[offset+1] == '{' {
				// start of a media file.
				if i := bytes.Index(rawTextBytes[offset:], []byte{'}', '}'}); i != -1 {
					endCurrentEffect(c, &effectBytes, currentEffect)
					currentEffect = 0
					parseMedia(c, rawTextBytes[offset+2:offset+i])
					offset += (i + 2)
				}
			}
		default:
			effectBytes = append(effectBytes, ch)
			offset += 1
		}
	}

	//fixup for links.
	fixupLinks(c)
}

func parseLink(c *ParaContext, linkBytes []byte) {
	if i := bytes.IndexByte(linkBytes, '|'); i != -1 {
		c.InnerContexts = append(c.InnerContexts, HyperLinkContext{
			BaseInlineContext: BaseInlineContext{BaseContext{parent: *c}},
			Text:              string(linkBytes[i+1:]),
			HyperLink:         string(linkBytes[:i]),
		})
	} else {
		// internal link
		c.InnerContexts = append(c.InnerContexts, HyperLinkContext{
			BaseInlineContext: BaseInlineContext{BaseContext{parent: *c}},
			Text:              string(linkBytes),
			IsInternal:        true,
		})
	}
}

func parseMedia(c *ParaContext, mediaBytes []byte) {
	mc := MediaContext{
		BaseInlineContext: BaseInlineContext{BaseContext{parent: *c}},
	}

	bytesLeft := mediaBytes
	if i := bytes.IndexByte(mediaBytes, '|'); i != -1 {
		mc.Title = string(mediaBytes[i+1:])
		bytesLeft = mediaBytes[:i]
	}

	if bytesLeft[0] == ' ' {
		mc.Align = AlignLeft
		bytesLeft = bytesLeft[1:]
	} else if bytesLeft[len(bytesLeft)-1] == ' ' {
		mc.Align = AlignRight
		bytesLeft = bytesLeft[:len(bytesLeft)-1]
	} else {
		mc.Align = AlignCenter
	}

	groups := validMedia.FindSubmatch(bytesLeft)
	if groups != nil && len(groups[2]) > 0 {
		dimentions := groups[2][1 : len(groups[2])-1]
		if i := bytes.Index(dimentions, []byte{'x'}); i != -1 {
			mc.Width, _ = strconv.ParseInt(string(dimentions[:i]), 10, 64)
			mc.Height, _ = strconv.ParseInt(string(dimentions[i+1:]), 10, 64)
		} else {
			mc.Width, _ = strconv.ParseInt(string(dimentions), 10, 64)
		}
	}
	mc.MediaResouce = string(groups[1])

	c.InnerContexts = append(c.InnerContexts, mc)
}

func endCurrentEffect(c *ParaContext, effectBytes *[]byte, currentEffect uint32) {
	if len(*effectBytes) == 0 {
		return
	}

	c.InnerContexts = append(c.InnerContexts, TextEffectContext{
		BaseInlineContext: BaseInlineContext{BaseContext{parent: *c}},
		EffectType:        currentEffect,
		Text:              string(*effectBytes),
	})
	*effectBytes = make([]byte, 0)
}

// returns the section header level, 0 means not a header.
func parseSectionHeader(line []byte) (int, []byte) {
	lineString := strings.TrimRight(string(line), "\t ")
	groups := validSectionHeader.FindStringSubmatch(lineString)
	if groups != nil && (len(groups[1]) == len(groups[3])) {
		return len(groups[1]), []byte(strings.TrimSpace(groups[2]))
	}
	return 0, nil
}

func fixupLinks(c *ParaContext) {
	for {
		if scanParaOnce(c) == false {
			return
		}
	}
}

// scanparaonce returns false when there is no links found.
func scanParaOnce(c *ParaContext) bool {
	for i := 0; i < len(c.InnerContexts); i++ {
		if tc, ok := c.InnerContexts[i].(TextEffectContext); ok {
			groups := validURL.FindStringSubmatchIndex(tc.Text)
			if groups != nil {
				newContenxts := make([]InlineContext, 0)
				before := []byte(tc.Text)[:groups[0]]
				if len(bytes.TrimSpace(before)) > 0 {
					newContenxts = append(newContenxts, TextEffectContext{
						BaseInlineContext: BaseInlineContext{BaseContext{parent: *c}},
						EffectType:        tc.EffectType,
						Text:              string(before),
					})
				}
				newContenxts = append(newContenxts, HyperLinkContext{
					BaseInlineContext: BaseInlineContext{BaseContext{parent: *c}},
					Text:              string([]byte(tc.Text)[groups[0]:groups[1]]),
					HyperLink:         string([]byte(tc.Text)[groups[0]:groups[1]]),
				})
				after := []byte(tc.Text)[groups[1]:]
				if len(bytes.TrimSpace(after)) > 0 {
					newContenxts = append(newContenxts, TextEffectContext{
						BaseInlineContext: BaseInlineContext{BaseContext{parent: *c}},
						EffectType:        tc.EffectType,
						Text:              string(after),
					})
				}
				if i < len(c.InnerContexts)-1 {
					c.InnerContexts = append(append(c.InnerContexts[:i], newContenxts...), c.InnerContexts[i+1:]...)
				} else {
					c.InnerContexts = append(c.InnerContexts[:i], newContenxts...)
				}
				return true
			}
		}
	}
	return false
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
