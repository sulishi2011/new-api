package dto

type Notify struct {
	Type    string        `json:"type"`
	Title   string        `json:"title"`
	Content string        `json:"content"`
	Values  []interface{} `json:"values"`
	Fields  []NotifyField `json:"fields,omitempty"`
}

type NotifyField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

const ContentValueParam = "{{value}}"

const (
	NotifyTypeQuotaExceed           = "quota_exceed"
	NotifyTypeChannelUpdate         = "channel_update"
	NotifyTypeChannelTest           = "channel_test"
	NotifyTypeChannelRequestFailure = "channel_request_failure"
	NotifyTypeChannelDisabled       = "channel_disabled"
)

func NewNotify(t string, title string, content string, values []interface{}) Notify {
	return Notify{
		Type:    t,
		Title:   title,
		Content: content,
		Values:  values,
	}
}

func NewNotifyWithFields(t string, title string, content string, values []interface{}, fields []NotifyField) Notify {
	notify := NewNotify(t, title, content, values)
	notify.Fields = fields
	return notify
}
