package expr

type node interface {
	nodeType() string
}

type literalNode struct {
	value Value
}

func (n *literalNode) nodeType() string { return "literal" }

type identNode struct {
	name string
}

func (n *identNode) nodeType() string { return "ident" }

type unaryNode struct {
	op      string
	operand node
}

func (n *unaryNode) nodeType() string { return "unary" }

type binaryNode struct {
	op    string
	left  node
	right node
}

func (n *binaryNode) nodeType() string { return "binary" }

type ternaryNode struct {
	cond       node
	consequent node
	alternate  node
}

func (n *ternaryNode) nodeType() string { return "ternary" }

type memberNode struct {
	object   node
	property string
}

func (n *memberNode) nodeType() string { return "member" }

type indexNode struct {
	object node
	index  node
}

func (n *indexNode) nodeType() string { return "index" }

type callNode struct {
	callee node
	args   []node
}

func (n *callNode) nodeType() string { return "call" }

type arrayNode struct {
	elements []node
}

func (n *arrayNode) nodeType() string { return "array" }

type objectNode struct {
	keys   []string
	values []node
}

func (n *objectNode) nodeType() string { return "object" }
