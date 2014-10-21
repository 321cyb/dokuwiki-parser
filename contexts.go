package dokuwiki

const (
	MediaTypePicture = iota
	MediaTypeVideo
	MediaTypeAudio
	MediaTypeFlash
)

const (
	AlignLeft = iota
	AlignCenter
	AlignRight
)

const (
	TextEffectBold        = 1 << iota
	TextEffectItalic      = 1 << iota
	TextEffectUnderline   = 1 << iota
	TextEffectMonoSpace   = 1 << iota
	TextEffectSubScript   = 1 << iota
	TextEffectSuperScript = 1 << iota
	TextEffectDeleted     = 1 << iota
)

type Context interface {
	GetParentContext() *Context
}

type BaseContext struct {
	parent *Context
}

func (p *BaseContext) GetParentContext() *Context {
	return p.parent
}

type ParseUnit struct {
	BaseContext
	Title         string
	InnerContexts []*Context
}

// Could be either block or inline context.
type NoWikiContext struct {
	BaseContext
	IsBlock bool
	Text    string
}

type HTMLContext struct {
	BaseContext
	Text string
}

// Block Contexts

// SectionHeaderContext can have bold or other text effect in it, nor links.
type SectionHeaderContext struct {
	BaseContext
	HeaderLevel int
	Text        string
}

type ListContext struct {
	BaseContext
	Ordered       bool
	InnerContexts []*Context
}

// Footer can contain: Text Effect, Links, Media FIles, NoWiki, Code
type FooterContext struct {
	BaseContext
	Content *TransientBlockContext
}

type CodeFileContext struct {
	BaseContext
	IsCode bool
	Text   string
	// Only valid when IsCode is false
	FileName string
}

// Transientblockcontext is a fake block context that is created to contain inline blocks.
type TransientBlockContext struct {
	BaseContext
	InnerContexts []*Context
}

// Inline Contexts

//Hyperlink text should not have effects.
type HyperLinkContext struct {
	BaseContext
	HyperLink string
	Text      string
}

type MediaContext struct {
	BaseContext
	MediaType    int
	Width        int
	Height       int
	Align        int
	Title        string
	MediaResouce string
}

type TextEffectContext struct {
	BaseContext
	EffectType uint32
	Text       string
}
