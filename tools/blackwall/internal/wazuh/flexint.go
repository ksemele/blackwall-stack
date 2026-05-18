package wazuh

import (
	"bytes"
	"fmt"
	"strconv"
)

// FlexInt decodes JSON numbers that may be quoted as strings.
// Wazuh syscollector returns some numeric fields (notably pid in processes)
// as JSON strings, while other numeric fields like ppid arrive as integers.
// FlexInt accepts both shapes and stores the value as int64.
type FlexInt int64

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*f = 0
		return nil
	}
	b = bytes.Trim(b, "\"")
	if len(b) == 0 {
		*f = 0
		return nil
	}
	n, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return fmt.Errorf("FlexInt: %w", err)
	}
	*f = FlexInt(n)
	return nil
}
