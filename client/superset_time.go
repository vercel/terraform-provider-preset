package client

import (
	"fmt"
	"strings"
	"time"
)

type SupersetTime time.Time

func (t *SupersetTime) UnmarshalJSON(data []byte) error {
	ts := strings.Trim(string(data), "\"")
	j, err := time.Parse(time.RFC3339, fmt.Sprintf("%sZ", ts[0:19]))

	if err != nil {
		return err
	}

	*t = SupersetTime(j)

	return nil
}
