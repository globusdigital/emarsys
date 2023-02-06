package emarsys

import (
	"encoding/json"
	"fmt"
)

type ResponseEnvelope struct {
	HTTPStatusCode int             `json:"httpStatusCode"`
	ReplyCode      int             `json:"replyCode"` // https://dev.emarsys.com/docs/emarsys-api/ZG9jOjI0ODk5NzY4-http-200-errors
	ReplyText      string          `json:"replyText"`
	Data           json.RawMessage `json:"data"`
	UnmarshalErr   error           `json:"-"`
}

func (s *ResponseEnvelope) Error() string {
	if s == nil {
		return ""
	}
	if s.UnmarshalErr != nil {
		return s.UnmarshalErr.Error()
	}
	if s.ReplyCode == 0 {
		return ""
	}
	return fmt.Sprintf(
		"ReplyCode:%d ReplyText:%q, HasData:%t",
		s.ReplyCode,
		s.ReplyText,
		s.Data != nil,
	)
}
