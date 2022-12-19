package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

// # cd fifa-update
// # go mod tidy && go build . && ./fifa-update

const DateTimBarFormat = "2006-01-02 15:04:05"

// FifaApi 数据源 api 的地址
const FifaApi = "http://apis.juhe.cn/fapigw/worldcup2022/schedule?type=&key=xxxxxxxxx"

// RobotApi 推送赛况通知的企业微信机器人 Api
const RobotApi = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxxxxxx"

// ErrReportApi 推送错误通知的企业微信机器人 Api(可以和上面的一致)
const ErrReportApi = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxxxxxx"

var timeMap = []string{ // 免费接口调用次数要少于 50 次, 首次启动程序需要浪费一次调用初始化数据
	/*
		// 第一场比赛 (0：00～1：45)
		"12:00AM", "12:15AM", "12:30AM", "12:45AM",
		"1:00AM", "1:15AM", "1:30AM", "1:45AM",
		"2:00AM", "2:15AM", "2:30AM", "2:45AM",

		// 第二场比赛（03：00～04：45）
		"3:00AM", "3:15AM", "3:30AM", "3:45AM",
		"4:00AM", "4:15AM", "4:30AM", "4:45AM",
		"5:00AM", "5:15AM", "5:30AM", "5:45AM",

		// 第三场比赛（18：00～19：45）
		"6:00PM", "6:15PM", "6:30PM", "6:45PM",
		"7:00PM", "7:15PM", "7:30PM", "7:45PM",
		"8:00PM", "8:15PM", "8:30PM", "8:45PM",

		// 第四场比赛（21：00～22：45）
		"9:00PM", "9:15PM", "9:30PM", "9:45PM",
		"10:00PM", "10:15PM", "10:30PM", "10:45PM",
		"11:00PM", "11:15PM", "11:30PM", "11:45PM",
	*/

	// 第一场比赛
	"11:00PM", "11:15PM", "11:30PM", "11:45PM",
	"12:00AM", "12:15AM", "12:30AM", "12:45AM",
	"1:00AM", "1:15AM", "1:30AM", "1:45AM",
	"2:00AM", "2:15AM", "2:30AM", "2:45AM",

	// 第二场比赛
	"3:00AM", "3:15AM", "3:30AM", "3:45AM",
	"4:00AM", "4:15AM", "4:30AM", "4:45AM",
	"5:00AM", "5:15AM", "5:30AM", "5:45AM",
	"6:00AM", "6:15AM", "6:30AM", "6:45AM",

	// Backup
	"7:00AM", "7:15AM", "7:30AM", "7:45AM",
}

// map[teamID] = [matchStatus-HostReamScore-GuestTeamScore]
var localMap map[string]string

func main() {
	refreshData()

	GoWithRecovery(func() {
		ticker := time.Tick(time.Second * time.Duration(60))
		for {
			select {
			case <-ticker:
				refreshData()
			}
		}
	})

	select {}

}

func checkIsTime() bool {

	if needInit() {
		return true
	}

	now := time.Now().Format(time.Kitchen)
	for _, t := range timeMap {
		if t == now {
			log.Printf("\n===\n命中检查时间[%s]", t)
			return true
		}
	}

	//log.Printf("当前[%s]未命中检查时间，再等等吧", now)
	fmt.Print(".")
	return false
}

func refreshData() {
	var err error
	defer func() {
		if err != nil {
			log.Printf("err[%s]", err.Error())
			httpPostJson(ErrReportApi, getErrStr(err))
		}
	}()

	if !checkIsTime() {
		return
	}

	fifa, err := grabFifa()
	if err != nil {
		return
	}

	needPush, diffData, err := diffLocal(fifa)
	if err != nil {
		return
	}

	if !needPush {
		log.Println("无需推送，跳过。")
		return
	}

	err = notifyWeCom(diffData)
	if err != nil {
		return
	}

	return
}

// grabFifa 拉数据
func grabFifa() (result []*FifaData, err error) {
	result = make([]*FifaData, 0)
	getData, err := httpGetJson(FifaApi)
	if err != nil {
		return
	}
	// demo test struct
	//get := `{"reason":"查询成功","result":{"data":[{"schedule_date":"2022-11-21","schedule_date_format":"11月21日","schedule_week":"周一","schedule_current":"0","schedule_list":[{"team_id":"1","date":"2022-11-21","date_time":"2022-11-21 00:00:00","host_team_id":"3","guest_team_id":"1","host_team_name":"卡塔尔","guest_team_name":"厄瓜多尔","host_team_score":"0","guest_team_score":"2","match_status":"3","match_des":"完赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"A","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/A3.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/A1.png"},{"team_id":"2","date":"2022-11-21","date_time":"2022-11-21 21:00:00","host_team_id":"5","guest_team_id":"6","host_team_name":"英格兰","guest_team_name":"伊朗","host_team_score":"6","guest_team_score":"2","match_status":"3","match_des":"完赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"B","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/B5.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/B6.png"}]},{"schedule_date":"2022-11-22","schedule_date_format":"11月22日","schedule_week":"周二","schedule_current":"0","schedule_list":[{"team_id":"3","date":"2022-11-22","date_time":"2022-11-22 00:00:00","host_team_id":"4","guest_team_id":"2","host_team_name":"塞内加尔","guest_team_name":"荷兰","host_team_score":"0","guest_team_score":"2","match_status":"3","match_des":"完赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"A","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/A4.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/A2.png"},{"team_id":"4","date":"2022-11-22","date_time":"2022-11-22 03:00:00","host_team_id":"7","guest_team_id":"8","host_team_name":"美国","guest_team_name":"威尔士","host_team_score":"1","guest_team_score":"1","match_status":"3","match_des":"完赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"B","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/B7.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/B8.png"},{"team_id":"5","date":"2022-11-22","date_time":"2022-11-22 18:00:00","host_team_id":"9","guest_team_id":"12","host_team_name":"阿根廷","guest_team_name":"沙特阿拉伯","host_team_score":"1","guest_team_score":"2","match_status":"3","match_des":"完赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"C","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/C9.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/C12.png"},{"team_id":"6","date":"2022-11-22","date_time":"2022-11-22 21:00:00","host_team_id":"14","guest_team_id":"16","host_team_name":"丹麦","guest_team_name":"突尼斯","host_team_score":"0","guest_team_score":"0","match_status":"3","match_des":"完赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"D","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/D14.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/D16.png"}]},{"schedule_date":"2022-11-23","schedule_date_format":"11月23日","schedule_week":"周三","schedule_current":"0","schedule_list":[{"team_id":"7","date":"2022-11-23","date_time":"2022-11-23 00:00:00","host_team_id":"10","guest_team_id":"11","host_team_name":"墨西哥","guest_team_name":"波兰","host_team_score":"0","guest_team_score":"0","match_status":"3","match_des":"完赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"C","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/C10.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/C11.png"},{"team_id":"8","date":"2022-11-23","date_time":"2022-11-23 03:00:00","host_team_id":"15","guest_team_id":"13","host_team_name":"法国","guest_team_name":"澳大利亚","host_team_score":"4","guest_team_score":"1","match_status":"3","match_des":"完赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"D","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/D15.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/D13.png"},{"team_id":"9","date":"2022-11-23","date_time":"2022-11-23 18:00:00","host_team_id":"24","guest_team_id":"23","host_team_name":"摩洛哥","guest_team_name":"克罗地亚","host_team_score":"0","guest_team_score":"0","match_status":"3","match_des":"完赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"F","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/F24.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/F23.png"},{"team_id":"10","date":"2022-11-23","date_time":"2022-11-23 21:00:00","host_team_id":"18","guest_team_id":"19","host_team_name":"德国","guest_team_name":"日本","host_team_score":"1","guest_team_score":"2","match_status":"3","match_des":"完赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"E","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/E18.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/E19.png"}]},{"schedule_date":"2022-11-24","schedule_date_format":"11月24日","schedule_week":"周四","schedule_current":"1","schedule_list":[{"team_id":"11","date":"2022-11-24","date_time":"2022-11-24 00:00:00","host_team_id":"20","guest_team_id":"17","host_team_name":"西班牙","guest_team_name":"哥斯达黎加","host_team_score":"7","guest_team_score":"0","match_status":"3","match_des":"完赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"E","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/E20.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/E17.png"},{"team_id":"12","date":"2022-11-24","date_time":"2022-11-24 03:00:00","host_team_id":"21","guest_team_id":"22","host_team_name":"比利时","guest_team_name":"加拿大","host_team_score":"1","guest_team_score":"0","match_status":"3","match_des":"完赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"F","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/F21.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/F22.png"},{"team_id":"13","date":"2022-11-24","date_time":"2022-11-24 18:00:00","host_team_id":"28","guest_team_id":"26","host_team_name":"瑞士","guest_team_name":"喀麦隆","host_team_score":"1","guest_team_score":"0","match_status":"2","match_des":"进行中","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"G","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/G28.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/G26.png"},{"team_id":"14","date":"2022-11-24","date_time":"2022-11-24 21:00:00","host_team_id":"32","guest_team_id":"31","host_team_name":"乌拉圭","guest_team_name":"韩国","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"H","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/H32.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/H31.png"}]},{"schedule_date":"2022-11-25","schedule_date_format":"11月25日","schedule_week":"周五","schedule_current":"0","schedule_list":[{"team_id":"15","date":"2022-11-25","date_time":"2022-11-25 00:00:00","host_team_id":"30","guest_team_id":"29","host_team_name":"葡萄牙","guest_team_name":"加纳","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"H","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/H30.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/H29.png"},{"team_id":"16","date":"2022-11-25","date_time":"2022-11-25 03:00:00","host_team_id":"25","guest_team_id":"27","host_team_name":"巴西","guest_team_name":"塞尔维亚","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第1轮","group_name":"G","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/G25.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/G27.png"},{"team_id":"17","date":"2022-11-25","date_time":"2022-11-25 18:00:00","host_team_id":"8","guest_team_id":"6","host_team_name":"威尔士","guest_team_name":"伊朗","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"B","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/B8.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/B6.png"},{"team_id":"18","date":"2022-11-25","date_time":"2022-11-25 21:00:00","host_team_id":"3","guest_team_id":"4","host_team_name":"卡塔尔","guest_team_name":"塞内加尔","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"A","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/A3.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/A4.png"}]},{"schedule_date":"2022-11-26","schedule_date_format":"11月26日","schedule_week":"周六","schedule_current":"0","schedule_list":[{"team_id":"19","date":"2022-11-26","date_time":"2022-11-26 00:00:00","host_team_id":"2","guest_team_id":"1","host_team_name":"荷兰","guest_team_name":"厄瓜多尔","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"A","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/A2.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/A1.png"},{"team_id":"20","date":"2022-11-26","date_time":"2022-11-26 03:00:00","host_team_id":"5","guest_team_id":"7","host_team_name":"英格兰","guest_team_name":"美国","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"B","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/B5.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/B7.png"},{"team_id":"21","date":"2022-11-26","date_time":"2022-11-26 18:00:00","host_team_id":"16","guest_team_id":"13","host_team_name":"突尼斯","guest_team_name":"澳大利亚","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"D","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/D16.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/D13.png"},{"team_id":"22","date":"2022-11-26","date_time":"2022-11-26 21:00:00","host_team_id":"11","guest_team_id":"12","host_team_name":"波兰","guest_team_name":"沙特阿拉伯","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"C","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/C11.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/C12.png"}]},{"schedule_date":"2022-11-27","schedule_date_format":"11月27日","schedule_week":"周日","schedule_current":"0","schedule_list":[{"team_id":"23","date":"2022-11-27","date_time":"2022-11-27 00:00:00","host_team_id":"15","guest_team_id":"14","host_team_name":"法国","guest_team_name":"丹麦","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"D","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/D15.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/D14.png"},{"team_id":"24","date":"2022-11-27","date_time":"2022-11-27 03:00:00","host_team_id":"9","guest_team_id":"10","host_team_name":"阿根廷","guest_team_name":"墨西哥","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"C","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/C9.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/C10.png"},{"team_id":"25","date":"2022-11-27","date_time":"2022-11-27 18:00:00","host_team_id":"19","guest_team_id":"17","host_team_name":"日本","guest_team_name":"哥斯达黎加","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"E","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/E19.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/E17.png"},{"team_id":"26","date":"2022-11-27","date_time":"2022-11-27 21:00:00","host_team_id":"21","guest_team_id":"24","host_team_name":"比利时","guest_team_name":"摩洛哥","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"F","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/F21.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/F24.png"}]},{"schedule_date":"2022-11-28","schedule_date_format":"11月28日","schedule_week":"周一","schedule_current":"0","schedule_list":[{"team_id":"27","date":"2022-11-28","date_time":"2022-11-28 00:00:00","host_team_id":"23","guest_team_id":"22","host_team_name":"克罗地亚","guest_team_name":"加拿大","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"F","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/F23.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/F22.png"},{"team_id":"28","date":"2022-11-28","date_time":"2022-11-28 03:00:00","host_team_id":"20","guest_team_id":"18","host_team_name":"西班牙","guest_team_name":"德国","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"E","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/E20.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/E18.png"},{"team_id":"29","date":"2022-11-28","date_time":"2022-11-28 18:00:00","host_team_id":"26","guest_team_id":"27","host_team_name":"喀麦隆","guest_team_name":"塞尔维亚","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"G","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/G26.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/G27.png"},{"team_id":"30","date":"2022-11-28","date_time":"2022-11-28 21:00:00","host_team_id":"31","guest_team_id":"29","host_team_name":"韩国","guest_team_name":"加纳","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"H","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/H31.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/H29.png"}]},{"schedule_date":"2022-11-29","schedule_date_format":"11月29日","schedule_week":"周二","schedule_current":"0","schedule_list":[{"team_id":"31","date":"2022-11-29","date_time":"2022-11-29 00:00:00","host_team_id":"25","guest_team_id":"28","host_team_name":"巴西","guest_team_name":"瑞士","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"G","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/G25.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/G28.png"},{"team_id":"32","date":"2022-11-29","date_time":"2022-11-29 03:00:00","host_team_id":"30","guest_team_id":"32","host_team_name":"葡萄牙","guest_team_name":"乌拉圭","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第2轮","group_name":"H","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/H30.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/H32.png"},{"team_id":"33","date":"2022-11-29","date_time":"2022-11-29 23:00:00","host_team_id":"2","guest_team_id":"3","host_team_name":"荷兰","guest_team_name":"卡塔尔","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"A","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/A2.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/A3.png"},{"team_id":"34","date":"2022-11-29","date_time":"2022-11-29 23:00:00","host_team_id":"1","guest_team_id":"4","host_team_name":"厄瓜多尔","guest_team_name":"塞内加尔","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"A","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/A1.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/A4.png"}]},{"schedule_date":"2022-11-30","schedule_date_format":"11月30日","schedule_week":"周三","schedule_current":"0","schedule_list":[{"team_id":"35","date":"2022-11-30","date_time":"2022-11-30 03:00:00","host_team_id":"8","guest_team_id":"5","host_team_name":"威尔士","guest_team_name":"英格兰","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"B","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/B8.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/B5.png"},{"team_id":"36","date":"2022-11-30","date_time":"2022-11-30 03:00:00","host_team_id":"6","guest_team_id":"7","host_team_name":"伊朗","guest_team_name":"美国","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"B","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/B6.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/B7.png"},{"team_id":"37","date":"2022-11-30","date_time":"2022-11-30 23:00:00","host_team_id":"16","guest_team_id":"15","host_team_name":"突尼斯","guest_team_name":"法国","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"D","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/D16.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/D15.png"},{"team_id":"38","date":"2022-11-30","date_time":"2022-11-30 23:00:00","host_team_id":"13","guest_team_id":"14","host_team_name":"澳大利亚","guest_team_name":"丹麦","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"D","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/D13.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/D14.png"}]},{"schedule_date":"2022-12-01","schedule_date_format":"12月01日","schedule_week":"周四","schedule_current":"0","schedule_list":[{"team_id":"39","date":"2022-12-01","date_time":"2022-12-01 03:00:00","host_team_id":"11","guest_team_id":"9","host_team_name":"波兰","guest_team_name":"阿根廷","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"C","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/C11.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/C9.png"},{"team_id":"40","date":"2022-12-01","date_time":"2022-12-01 03:00:00","host_team_id":"12","guest_team_id":"10","host_team_name":"沙特阿拉伯","guest_team_name":"墨西哥","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"C","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/C12.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/C10.png"},{"team_id":"41","date":"2022-12-01","date_time":"2022-12-01 23:00:00","host_team_id":"23","guest_team_id":"21","host_team_name":"克罗地亚","guest_team_name":"比利时","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"F","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/F23.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/F21.png"},{"team_id":"42","date":"2022-12-01","date_time":"2022-12-01 23:00:00","host_team_id":"22","guest_team_id":"24","host_team_name":"加拿大","guest_team_name":"摩洛哥","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"F","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/F22.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/F24.png"}]},{"schedule_date":"2022-12-02","schedule_date_format":"12月02日","schedule_week":"周五","schedule_current":"0","schedule_list":[{"team_id":"43","date":"2022-12-02","date_time":"2022-12-02 03:00:00","host_team_id":"19","guest_team_id":"20","host_team_name":"日本","guest_team_name":"西班牙","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"E","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/E19.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/E20.png"},{"team_id":"44","date":"2022-12-02","date_time":"2022-12-02 03:00:00","host_team_id":"17","guest_team_id":"18","host_team_name":"哥斯达黎加","guest_team_name":"德国","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"E","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/E17.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/E18.png"},{"team_id":"45","date":"2022-12-02","date_time":"2022-12-02 23:00:00","host_team_id":"31","guest_team_id":"30","host_team_name":"韩国","guest_team_name":"葡萄牙","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"H","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/H31.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/H30.png"},{"team_id":"46","date":"2022-12-02","date_time":"2022-12-02 23:00:00","host_team_id":"29","guest_team_id":"32","host_team_name":"加纳","guest_team_name":"乌拉圭","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"H","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/H29.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/H32.png"}]},{"schedule_date":"2022-12-03","schedule_date_format":"12月03日","schedule_week":"周六","schedule_current":"0","schedule_list":[{"team_id":"47","date":"2022-12-03","date_time":"2022-12-03 03:00:00","host_team_id":"27","guest_team_id":"28","host_team_name":"塞尔维亚","guest_team_name":"瑞士","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"G","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/G27.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/G28.png"},{"team_id":"48","date":"2022-12-03","date_time":"2022-12-03 03:00:00","host_team_id":"26","guest_team_id":"25","host_team_name":"喀麦隆","guest_team_name":"巴西","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"1","match_type_name":"小组赛","match_type_des":"第3轮","group_name":"G","host_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/G26.png","guest_team_logo_url":"https:\/\/juhe.oss-cn-hangzhou.aliyuncs.com\/api_image\/616\/worldcup2022\/G25.png"},{"team_id":"49","date":"2022-12-03","date_time":"2022-12-03 23:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"A组第1","guest_team_name":"B组第2","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"2","match_type_name":"1\/8决赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null}]},{"schedule_date":"2022-12-04","schedule_date_format":"12月04日","schedule_week":"周日","schedule_current":"0","schedule_list":[{"team_id":"50","date":"2022-12-04","date_time":"2022-12-04 03:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"C组第1","guest_team_name":"D组第2","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"2","match_type_name":"1\/8决赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null},{"team_id":"51","date":"2022-12-04","date_time":"2022-12-04 23:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"D组第1","guest_team_name":"C组第2","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"2","match_type_name":"1\/8决赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null}]},{"schedule_date":"2022-12-05","schedule_date_format":"12月05日","schedule_week":"周一","schedule_current":"0","schedule_list":[{"team_id":"52","date":"2022-12-05","date_time":"2022-12-05 03:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"B组第1","guest_team_name":"A组第2","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"2","match_type_name":"1\/8决赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null},{"team_id":"53","date":"2022-12-05","date_time":"2022-12-05 23:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"E组第1","guest_team_name":"F组第2","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"2","match_type_name":"1\/8决赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null}]},{"schedule_date":"2022-12-06","schedule_date_format":"12月06日","schedule_week":"周二","schedule_current":"0","schedule_list":[{"team_id":"54","date":"2022-12-06","date_time":"2022-12-06 03:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"G组第1","guest_team_name":"H组第2","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"2","match_type_name":"1\/8决赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null},{"team_id":"55","date":"2022-12-06","date_time":"2022-12-06 23:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"F组第1","guest_team_name":"E组第2","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"2","match_type_name":"1\/8决赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null}]},{"schedule_date":"2022-12-07","schedule_date_format":"12月07日","schedule_week":"周三","schedule_current":"0","schedule_list":[{"team_id":"56","date":"2022-12-07","date_time":"2022-12-07 03:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"H组第1","guest_team_name":"G组第2","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"2","match_type_name":"1\/8决赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null}]},{"schedule_date":"2022-12-09","schedule_date_format":"12月09日","schedule_week":"周五","schedule_current":"0","schedule_list":[{"team_id":"57","date":"2022-12-09","date_time":"2022-12-09 23:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"待定","guest_team_name":"待定","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"3","match_type_name":"1\/4决赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null}]},{"schedule_date":"2022-12-10","schedule_date_format":"12月10日","schedule_week":"周六","schedule_current":"0","schedule_list":[{"team_id":"58","date":"2022-12-10","date_time":"2022-12-10 03:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"待定","guest_team_name":"待定","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"3","match_type_name":"1\/4决赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null},{"team_id":"59","date":"2022-12-10","date_time":"2022-12-10 23:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"待定","guest_team_name":"待定","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"3","match_type_name":"1\/4决赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null}]},{"schedule_date":"2022-12-11","schedule_date_format":"12月11日","schedule_week":"周日","schedule_current":"0","schedule_list":[{"team_id":"60","date":"2022-12-11","date_time":"2022-12-11 03:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"待定","guest_team_name":"待定","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"3","match_type_name":"1\/4决赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null}]},{"schedule_date":"2022-12-14","schedule_date_format":"12月14日","schedule_week":"周三","schedule_current":"0","schedule_list":[{"team_id":"61","date":"2022-12-14","date_time":"2022-12-14 03:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"待定","guest_team_name":"待定","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"4","match_type_name":"半决赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null}]},{"schedule_date":"2022-12-15","schedule_date_format":"12月15日","schedule_week":"周四","schedule_current":"0","schedule_list":[{"team_id":"62","date":"2022-12-15","date_time":"2022-12-15 03:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"待定","guest_team_name":"待定","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"4","match_type_name":"半决赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null}]},{"schedule_date":"2022-12-17","schedule_date_format":"12月17日","schedule_week":"周六","schedule_current":"0","schedule_list":[{"team_id":"63","date":"2022-12-17","date_time":"2022-12-17 23:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"待定","guest_team_name":"待定","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"5","match_type_name":"季军赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null}]},{"schedule_date":"2022-12-18","schedule_date_format":"12月18日","schedule_week":"周日","schedule_current":"0","schedule_list":[{"team_id":"64","date":"2022-12-18","date_time":"2022-12-18 23:00:00","host_team_id":"0","guest_team_id":"0","host_team_name":"待定","guest_team_name":"待定","host_team_score":"-","guest_team_score":"-","match_status":"1","match_des":"未开赛","match_type":"6","match_type_name":"决赛","match_type_des":"","group_name":"","host_team_logo_url":null,"guest_team_logo_url":null}]}],"ext":{"current_match_type":"1","current_match_type_des":"小组赛"}},"error_code":0}`
	//getData := []byte(get)

	fifa := &Fifa{}
	err = json.Unmarshal(getData, fifa)
	if err != nil {
		return
	}

	if fifa.ErrorCode != 0 {
		err = errors.New("获取 fifa 数据失败,errCode 不为0, reason:" + fifa.Reason)
	}

	data := fifa.Result.Data
	result = data
	return
}

// diffLocal 比较差异
func diffLocal(input []*FifaData) (needPush bool, diffData []*FifaScheduleList, err error) {
	isInit := needInit()
	if isInit {
		err = initLocalData(input)
		return
	}

	diffData = make([]*FifaScheduleList, 0)
	for _, schedule := range input {
		for _, race := range schedule.ScheduleList {
			// 如果还没到对应比赛时间，跳过检查
			raceDate, _ := time.ParseInLocation(DateTimBarFormat, race.DateTime, time.Local)
			if time.Now().Before(raceDate.Add(-1 * time.Second)) {
				continue
			}
			// 判断本地数据是否与在线数据相符
			localData, ok := localMap[getKey(race)]
			if !ok || localData != getValue(race) {
				// 插入变更数据
				diffData = append(diffData, race)
				// 更新数据
				localMap[getKey(race)] = getValue(race)
				continue
			}
		}
	}

	if !isInit && len(diffData) > 0 {
		needPush = true
	}

	return
}

func notifyWeCom(input []*FifaScheduleList) (err error) {

	for _, race := range input {
		weComPush := makePush(race)
		weComPushByte, _ := json.Marshal(weComPush)
		httpPostJson(RobotApi, weComPushByte)
		log.Printf("推送更新：[%s][%s]%s->%s,[%s]%s-%s",
			race.Date, race.TeamID, race.HostTeamName, race.GuestTeamName,
			race.MatchDes, race.HostTeamScore, race.GuestTeamScore)
	}

	return
}

func makePush(data *FifaScheduleList) (result *WeComPush) {
	result = &WeComPush{
		Msgtype: "template_card",
		TemplateCard: &TemplateCard{
			CardType: "text_notice",
			Source: &Source{
				IconURL:   "", // 主场国旗
				Desc:      "世界杯赛况",
				DescColor: 0,
			},
			MainTitle: &MainTitle{
				Title: "【赛况更新】",
				Desc:  "", // 【小组赛】第几轮-A
			},
			EmphasisContent: &EmphasisContent{
				Title: "", // 比分
				Desc:  "", // 比赛状态
			},
			SubTitleText: "", // 【小组赛】第几轮-A
			HorizontalContentList: []*HorizontalContentList{
				{
					Keyname: "温馨提示",
					Value:   "赛况每15分钟刷新",
				},
				{
					Keyname: "特别提示",
					Value:   "免费数据源不保证实时",
				},
				{
					Keyname: "当前时间",
					Value:   time.Now().Format("2006-01-02 15:04:05"),
				},
				// 追加一个比赛时间
			},
			CardAction: &CardAction{
				Type:  2,
				URL:   "",
				AppID: "wxc3435ec8eb22c84f", // 点开卡片跳去腾讯体育看比分
			},
		},
	}

	card := result.TemplateCard
	card.MainTitle.Title += fmt.Sprintf("%svs%s", data.HostTeamName, data.GuestTeamName)
	card.MainTitle.Desc = fmt.Sprintf("【%s】%s%s组", data.MatchTypeName, data.MatchTypeDes, data.GroupName)
	card.Source.IconURL = data.HostTeamLogoURL
	card.EmphasisContent.Title = fmt.Sprintf("%s : %s", data.HostTeamScore, data.GuestTeamScore)
	card.EmphasisContent.Desc = fmt.Sprintf("【%s】",
		data.MatchDes)
	card.SubTitleText = fmt.Sprintf("👏 预祝和你想得一样!")
	card.HorizontalContentList = append(card.HorizontalContentList, &HorizontalContentList{
		Keyname: "开场时间",
		Value:   data.DateTime,
	})

	raceDatetime, _ := time.ParseInLocation(DateTimBarFormat, data.DateTime, time.Local)
	raceBeginPeriod := time.Now().Sub(raceDatetime).Minutes()
	if raceBeginPeriod >= 2*60 {
		card.HorizontalContentList = append(card.HorizontalContentList, &HorizontalContentList{
			Keyname: "【备注】",
			Value:   "距离时间过长，需核实准确性",
		})
	}
	card.HorizontalContentList = append(card.HorizontalContentList, &HorizontalContentList{
		Keyname: "距离开场已经",
		Value:   fmt.Sprintf("%.1f分钟", raceBeginPeriod),
	})

	return
}

func needInit() bool {
	if localMap == nil {
		log.Printf("数据需要进行初始化。")
		return true
	}
	return false
}

func getKey(race *FifaScheduleList) string {
	return race.TeamID
}

func getValue(race *FifaScheduleList) string {
	return strings.Join([]string{
		race.MatchStatus,
		race.HostTeamScore,
		race.GuestTeamScore,
	}, "-")
}

func initLocalData(input []*FifaData) (err error) {
	localMap = make(map[string]string, 0)
	if input == nil || len(input) == 0 {
		log.Printf("数据源无数据，完成初始化。")
		return
	}

	for _, schedule := range input {
		for _, race := range schedule.ScheduleList {
			localMap[getKey(race)] = getValue(race)

			log.Printf("初始化：[%s][%s]%s->%s,[%s]%s-%s",
				race.Date, race.TeamID, race.HostTeamName, race.GuestTeamName,
				race.MatchDes, race.HostTeamScore, race.GuestTeamScore)

		}
	}

	log.Printf("数据完成初始化。")
	return
}

func httpGetJson(url string) (result []byte, err error) {
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("getErr, url[%s] err[%s]", url, err.Error())
		return
	}
	defer resp.Body.Close()

	//statusCode := resp.StatusCode
	//hea := resp.Header
	body, _ := ioutil.ReadAll(resp.Body)

	//log.Printf("get done, code[%d] header[%+v], body[%s]",
	//	statusCode, hea, string(body))

	result = body
	return
}

func httpPostJson(url string, msg []byte) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(msg))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        0,
			MaxIdleConnsPerHost: 3000,
			IdleConnTimeout:     600 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			ResponseHeaderTimeout: 60 * time.Second,
		},
		Timeout: 5 * 60 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("updateWorldCupRank, doPost, err[%s]", err.Error())
		return
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode
	hea := resp.Header
	body, _ := ioutil.ReadAll(resp.Body)

	if statusCode != 200 {
		log.Printf("updateWorldCupRank, doPost, req[%s] code[%d] header[%+v], body[%s]",
			string(msg), statusCode, hea, string(body))
	}
	return
}

func getErrStr(err error) []byte {
	type postStruct struct {
		Msgtype string `json:"msgtype"`
		Text    struct {
			Content string `json:"content"`
		} `json:"text"`
	}

	post := &postStruct{
		Msgtype: "text",
		Text: struct {
			Content string `json:"content"`
		}{Content: "异常\n---\n" + err.Error()},
	}

	send, _ := json.Marshal(post)
	return send
}
