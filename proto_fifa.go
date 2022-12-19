package main

type Fifa struct {
	Reason    string      `json:"reason"`
	Result    *FifaResult `json:"result"`
	ErrorCode int         `json:"error_code"`
}
type FifaScheduleList struct {
	TeamID           string `json:"team_id"`
	Date             string `json:"date"`
	DateTime         string `json:"date_time"`
	HostTeamID       string `json:"host_team_id"`
	GuestTeamID      string `json:"guest_team_id"`
	HostTeamName     string `json:"host_team_name"`
	GuestTeamName    string `json:"guest_team_name"`
	HostTeamScore    string `json:"host_team_score"`
	GuestTeamScore   string `json:"guest_team_score"`
	MatchStatus      string `json:"match_status"`
	MatchDes         string `json:"match_des"`
	MatchType        string `json:"match_type"`
	MatchTypeName    string `json:"match_type_name"`
	MatchTypeDes     string `json:"match_type_des"`
	GroupName        string `json:"group_name"`
	HostTeamLogoURL  string `json:"host_team_logo_url"`
	GuestTeamLogoURL string `json:"guest_team_logo_url"`
}
type FifaData struct {
	ScheduleDate       string              `json:"schedule_date"`
	ScheduleDateFormat string              `json:"schedule_date_format"`
	ScheduleWeek       string              `json:"schedule_week"`
	ScheduleCurrent    string              `json:"schedule_current"`
	ScheduleList       []*FifaScheduleList `json:"schedule_list"`
}
type FifaExt struct {
	CurrentMatchType    string `json:"current_match_type"`
	CurrentMatchTypeDes string `json:"current_match_type_des"`
}
type FifaResult struct {
	Data []*FifaData `json:"data"`
	Ext  *FifaExt    `json:"ext"`
}
