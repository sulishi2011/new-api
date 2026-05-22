package types

type ChannelError struct {
	ChannelId              int    `json:"channel_id"`
	ChannelType            int    `json:"channel_type"`
	ChannelName            string `json:"channel_name"`
	VendorProfileCode      string `json:"vendor_profile_code,omitempty"`
	AutoDisablePolicyGroup string `json:"auto_disable_policy_group,omitempty"`
	IsMultiKey             bool   `json:"is_multi_key"`
	AutoBan                bool   `json:"auto_ban"`
	UsingKey               string `json:"using_key"`
}

func NewChannelError(channelId int, channelType int, channelName string, isMultiKey bool, usingKey string, autoBan bool, metadata ...string) *ChannelError {
	channelError := &ChannelError{
		ChannelId:              channelId,
		ChannelType:            channelType,
		ChannelName:            channelName,
		IsMultiKey:             isMultiKey,
		AutoBan:                autoBan,
		UsingKey:               usingKey,
		VendorProfileCode:      "",
		AutoDisablePolicyGroup: "",
	}
	if len(metadata) > 0 {
		channelError.VendorProfileCode = metadata[0]
	}
	if len(metadata) > 1 {
		channelError.AutoDisablePolicyGroup = metadata[1]
	}
	return channelError
}
