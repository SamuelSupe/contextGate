package adapter

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/gocql/gocql"
	"github.com/shopspring/decimal"
	"gopkg.in/inf.v0"
)

// CQL supplies the prepared parameter's native type at marshal time. Converting
// JSON numbers earlier would lose decimal precision or confuse float and double.
type cqlParam struct{ value any }

func (p cqlParam) MarshalCQL(info gocql.TypeInfo) ([]byte, error) {
	v := p.value
	text, numeric := "", false
	switch n := v.(type) {
	case json.Number:
		text, numeric = string(n), true
	case string:
		text = n
	case []any:
		items := make([]any, len(n))
		for i, item := range n {
			items[i] = cqlParam{item}
		}
		return gocql.Marshal(info, items)
	case map[string]any:
		items := make(map[string]any, len(n))
		for k, item := range n {
			items[k] = cqlParam{item}
		}
		return gocql.Marshal(info, items)
	}
	if text != "" {
		switch info.Type() {
		case gocql.TypeDecimal:
			d, err := decimal.NewFromString(text)
			if err != nil || d.Exponent() == math.MinInt32 {
				return nil, fmt.Errorf("invalid CQL decimal parameter")
			}
			v = inf.NewDecBig(d.Coefficient(), inf.Scale(-d.Exponent()))
		case gocql.TypeFloat, gocql.TypeDouble:
			bits := 64
			if info.Type() == gocql.TypeFloat {
				bits = 32
			}
			n, err := strconv.ParseFloat(text, bits)
			if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
				return nil, fmt.Errorf("invalid CQL floating parameter")
			}
			v = n
			if bits == 32 {
				v = float32(n)
			}
		default:
			if numeric {
				v = text
			}
		}
	}
	return gocql.Marshal(info, v)
}
