package main

type WeComPush struct {
	Msgtype      string        `json:"msgtype"`
	TemplateCard *TemplateCard `json:"template_card"`
}
type Source struct {
	IconURL   string `json:"icon_url"`
	Desc      string `json:"desc"`
	DescColor int    `json:"desc_color"`
}
type MainTitle struct {
	Title string `json:"title"`
	Desc  string `json:"desc"`
}
type EmphasisContent struct {
	Title string `json:"title"`
	Desc  string `json:"desc"`
}
type HorizontalContentList struct {
	Keyname string `json:"keyname"`
	Value   string `json:"value"`
}
type CardAction struct {
	Type  int    `json:"type"`
	URL   string `json:"url"`
	AppID string `json:"appid"`
}
type TemplateCard struct {
	CardType              string                   `json:"card_type"`
	Source                *Source                  `json:"source"`
	MainTitle             *MainTitle               `json:"main_title"`
	EmphasisContent       *EmphasisContent         `json:"emphasis_content"`
	SubTitleText          string                   `json:"sub_title_text"`
	HorizontalContentList []*HorizontalContentList `json:"horizontal_content_list"`
	CardAction            *CardAction              `json:"card_action"`
}
