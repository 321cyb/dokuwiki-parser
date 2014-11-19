package dokuwiki

const (
	AlignLeft = iota
	AlignCenter
	AlignRight
)

const (
	TextEffectBold      = 1 << iota
	TextEffectItalic    = 1 << iota
	TextEffectUnderline = 1 << iota
	TextEffectMonoSpace = 1 << iota
)

type Context interface {
	GetParentContext() Context
	SetParentContext(Context)
}

type BaseContext struct {
	parent Context
}

func (b BaseContext) GetParentContext() Context {
	return b.parent
}

func (b BaseContext) SetParentContext(pContext Context) {
	b.parent = pContext
}

type ParseUnit struct {
	BaseContext
	Title    string
	Sections []BlockContext
}

type BlockContext interface {
	Context
	block()
}

type BaseBlockContext struct {
	BaseContext
}

func (b BaseBlockContext) block() {}

// SectionHeader can have bold or other text effect in it, nor links.
// SectionHeader should be the beginning of a line,  no whitespace before it.
type SectionHeaderContext struct {
	BaseBlockContext
	HeaderLevel int
	HeaderText  string
}

type ListContext struct {
	BaseBlockContext
	Level         int
	Ordered       bool
	InnerContexts []BlockContext
}

// ParaContext is a fake block context that is created to contain inline blocks.
type ParaContext struct {
	BaseBlockContext
	rawText       string
	InnerContexts []InlineContext
}

// Inline Contexts
type InlineContext interface {
	Context
	inline()
}

type BaseInlineContext struct {
	BaseContext
}

func (b BaseInlineContext) inline() {}

type NoWikiContext struct {
	BaseInlineContext
	Text string
}

type HTMLContext struct {
	BaseInlineContext
	Text string
}

type CodeFileContext struct {
	BaseInlineContext
	Text string
}

//Hyperlink text should not have effects.
type HyperLinkContext struct {
	BaseInlineContext
	HyperLink  string
	Text       string
	IsInternal bool
}

type MediaContext struct {
	BaseInlineContext
	Width        int64
	Height       int64
	Align        int
	Title        string
	MediaResouce string
}

type TextEffectContext struct {
	BaseInlineContext
	EffectType uint32
	Text       string
}
