package types

type ChannelError struct {
	ChannelId         int    `json:"channel_id"`
	ChannelType       int    `json:"channel_type"`
	ChannelName       string `json:"channel_name"`
	VendorProfileCode string `json:"vendor_profile_code,omitempty"`
	IsMultiKey        bool   `json:"is_multi_key"`
	AutoBan           bool   `json:"auto_ban"`
	UsingKey          string `json:"using_key"`
}

func NewChannelError(channelId int, channelType int, channelName string, isMultiKey bool, usingKey string, autoBan bool, vendorProfileCode ...string) *ChannelError {
	channelError := &ChannelError{
		ChannelId:         channelId,
		ChannelType:       channelType,
		ChannelName:       channelName,
		IsMultiKey:        isMultiKey,
		AutoBan:           autoBan,
		UsingKey:          usingKey,
		VendorProfileCode: "",
	}
	if len(vendorProfileCode) > 0 {
		channelError.VendorProfileCode = vendorProfileCode[0]
	}
	return channelError
}
