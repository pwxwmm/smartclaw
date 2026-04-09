package native

type Direction int

const (
	DirectionInherit Direction = iota
	DirectionLTR
	DirectionRTL
)

type FlexDirection int

const (
	FlexDirectionColumn FlexDirection = iota
	FlexDirectionColumnReverse
	FlexDirectionRow
	FlexDirectionRowReverse
)

type Justify int

const (
	JustifyFlexStart Justify = iota
	JustifyCenter
	JustifyFlexEnd
	JustifySpaceBetween
	JustifySpaceAround
	JustifySpaceEvenly
)

type Align int

const (
	AlignAuto Align = iota
	AlignFlexStart
	AlignCenter
	AlignFlexEnd
	AlignStretch
	AlignBaseline
	AlignSpaceBetween
	AlignSpaceAround
)

type Wrap int

const (
	WrapNoWrap Wrap = iota
	WrapWrap
	WrapWrapReverse
)

type Position int

const (
	PositionRelative Position = iota
	PositionAbsolute
)

type Edge int

const (
	EdgeLeft Edge = iota
	EdgeTop
	EdgeRight
	EdgeBottom
	EdgeStart
	EdgeEnd
	EdgeHorizontal
	EdgeVertical
	EdgeAll
)

type Display int

const (
	DisplayFlex Display = iota
	DisplayNone
)

type Overflow int

const (
	OverflowVisible Overflow = iota
	OverflowHidden
	OverflowScroll
)

type YogaNode struct {
	Width          float64
	Height         float64
	MinWidth       float64
	MinHeight      float64
	MaxWidth       float64
	MaxHeight      float64
	Direction      Direction
	FlexDirection  FlexDirection
	JustifyContent Justify
	AlignItems     Align
	AlignSelf      Align
	AlignContent   Align
	FlexWrap       Wrap
	Position       Position
	Display        Display
	Overflow       Overflow
	Flex           float64
	FlexGrow       float64
	FlexShrink     float64
	FlexBasis      float64
	AspectRatio    float64
	Margin         [9]float64
	Padding        [9]float64
	Border         [9]float64
	PositionEdge   [9]float64
	Children       []*YogaNode
	Parent         *YogaNode
	computed       bool
}

func NewYogaNode() *YogaNode {
	return &YogaNode{
		Direction:      DirectionInherit,
		FlexDirection:  FlexDirectionColumn,
		JustifyContent: JustifyFlexStart,
		AlignItems:     AlignStretch,
		AlignSelf:      AlignAuto,
		AlignContent:   AlignFlexStart,
		FlexWrap:       WrapNoWrap,
		Position:       PositionRelative,
		Display:        DisplayFlex,
		Overflow:       OverflowVisible,
		Flex:           0,
		FlexGrow:       0,
		FlexShrink:     1,
		FlexBasis:      -1,
		AspectRatio:    -1,
	}
}

func (n *YogaNode) InsertChild(child *YogaNode, index int) {
	child.Parent = n
	if index >= len(n.Children) {
		n.Children = append(n.Children, child)
	} else {
		n.Children = append(n.Children[:index], append([]*YogaNode{child}, n.Children[index:]...)...)
	}
}

func (n *YogaNode) RemoveChild(child *YogaNode) {
	for i, c := range n.Children {
		if c == child {
			n.Children = append(n.Children[:i], n.Children[i+1:]...)
			child.Parent = nil
			return
		}
	}
}

func (n *YogaNode) ChildCount() int {
	return len(n.Children)
}

func (n *YogaNode) SetWidth(width float64) {
	n.Width = width
}

func (n *YogaNode) SetHeight(height float64) {
	n.Height = height
}

func (n *YogaNode) SetMargin(edge Edge, value float64) {
	n.Margin[edge] = value
}

func (n *YogaNode) SetPadding(edge Edge, value float64) {
	n.Padding[edge] = value
}

func (n *YogaNode) SetBorder(edge Edge, value float64) {
	n.Border[edge] = value
}

func (n *YogaNode) SetPosition(edge Edge, value float64) {
	n.PositionEdge[edge] = value
}

func (n *YogaNode) CalculateLayout(width, height float64, direction Direction) {
	n.Width = width
	n.Height = height
	n.Direction = direction
	n.computed = true
}

func (n *YogaNode) GetComputedLeft() float64 {
	return n.PositionEdge[EdgeLeft]
}

func (n *YogaNode) GetComputedTop() float64 {
	return n.PositionEdge[EdgeTop]
}

func (n *YogaNode) GetComputedWidth() float64 {
	return n.Width
}

func (n *YogaNode) GetComputedHeight() float64 {
	return n.Height
}

func (n *YogaNode) GetComputedMargin(edge Edge) float64 {
	return n.Margin[edge]
}

func (n *YogaNode) GetComputedPadding(edge Edge) float64 {
	return n.Padding[edge]
}

func (n *YogaNode) GetComputedBorder(edge Edge) float64 {
	return n.Border[edge]
}

func (n *YogaNode) IsComputed() bool {
	return n.computed
}
