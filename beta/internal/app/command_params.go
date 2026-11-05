package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	// Keep in step with TAKEOFF_MIN_ALT / TAKEOFF_MAX_ALT in web/src/modes.ts.
	takeoffMinAlt = 2.0
	takeoffMaxAlt = 100.0
)

// ParamsError lists every problem with a command's body. The API turns it into 422 INVALID_PARAMS.
type ParamsError struct {
	Problems []string
}

func (e *ParamsError) Error() string {
	return strings.Join(e.Problems, ";")
}

// decodeStrict decodes body into dst (a pointer). Empty body = {}. Unknown keys are an error.
func decodeStrict(body []byte, dst any) error {
	if string(bytes.TrimSpace(body)) == "null" {
		return &ParamsError{Problems: []string{"body: must be a JSON object"}}
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return &ParamsError{Problems: []string{"body: " + err.Error()}}
	}
	return nil
}

// NoParams is arm/disarm: only an empty body or {} is allowed.
type NoParams struct{}

type TakeOffParams struct {
	Altitude *float64 `json:"altitude"`
}

func (p TakeOffParams) validate() error {
	var problems []string
	if p.Altitude == nil {
		problems = append(problems, "altitude: required")
	} else if *p.Altitude < takeoffMinAlt || *p.Altitude > takeoffMaxAlt {
		problems = append(problems, fmt.Sprintf("altitude: must be between %v and %v, got %v", takeoffMinAlt, takeoffMaxAlt, *p.Altitude))
	}
	if len(problems) > 0 {
		return &ParamsError{Problems: problems}
	}
	return nil
}
