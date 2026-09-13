package command

import "time"

const (
	KindSet      = "SET"
	KindGet      = "GET"
	KindDelete   = "DELETE"
	KindPing     = "PING"
	KindGetWhere = "GET_WHERE"
)

type Command struct {
	Kind            string
	Key             string
	Value           string
	TTL             time.Duration
	IsNumber        bool
	Field           string
	Operator        string
	Operand         string
	OperandIsNumber bool
}

type token struct {
	text   string
	quoted bool
}
