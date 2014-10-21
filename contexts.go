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
	GetParent() Context
	GetChildren() []Context
	AppendChild(Context)
}

type BaseContext struct {
	parent   Context
	children []Context
}

func (p *BaseContext) GetParent() Context {
	return p.parent
}

func (p *BaseContext) GetChildren() []Context {
	return p.children
}

func (p *BaseContext) AppendChild(c Context) {
	p.children = append(p.children, c)
}

// Could be either block or inline context.
type NoWikiContext struct {
	BaseContext
	Text string
}

type HTMLContext struct {
	BaseContext
	Text string
}

// Block Contexts
type SectionHeaderContext struct {
	BaseContext
	HeaderLevel int
	Text        string
}

type ListContext struct {
	BaseContext
	Ordered bool
}

type FooterContext struct {
	BaseContext
}

type CodeFileContext struct {
	BaseContext
	IsCode bool
	// Only valid when IsCode is false
	FileName string
	Text     string
}

//TODO: phase 2
type QuoteContext struct {
	BaseContext
}

//TODO: phase 2
type TableContext struct {
	BaseContext
}

// Inline Contexts
type HyperLinkContext struct {
	BaseContext
	HyperLink string
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
