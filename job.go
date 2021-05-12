package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// jd miaosha response type
type Group struct {
	Gid       uint8  `json:"gid"`
	GroupTime string `json:"groupTime"`
}
type Miaosha struct {
	ShortWname string `json:"shortWname"`
	WareId     string `json:"wareId"`
	// imageurl      string
	JdPrice       string `json:"jdPrice"`
	MiaoShaPrice  string `json:"miaoShaPrice"`
	StartTimeShow string `json:"startTimeShow"`
}
type MiaoshaListJson struct {
	Groups      []Group   `json:"groups"`
	MiaoShaList []Miaosha `json:"miaoShaList"`
	Gid         string    `json:"gid"`
}

// 过滤 5折 或者低于 15块 的商品
func FilterGoods(l []Miaosha, maxPrice float64, minDisCount float64) []Miaosha {
	var r []Miaosha
	for _, good := range l {
		jdPrice, _ := strconv.ParseFloat(good.MiaoShaPrice, 32)
		originPrice, _ := strconv.ParseFloat(good.JdPrice, 32)
		discount := jdPrice / originPrice
		if jdPrice < maxPrice || discount < minDisCount {
			r = append(r, good)
		}
	}
	return r
}
func GetMiaoshaList(gid uint8) MiaoshaListJson {
	v := url.Values{}
	v.Add("appid", "o2_channels")
	v.Add("functionId", "pcMiaoShaAreaList")
	v.Add("client", "pc")
	v.Add("clientVersion", "1.0.0")
	v.Add("callback", "pcMiaoShaAreaList")
	v.Add("jsonp", "pcMiaoShaAreaList")
	if gid == 0 {
		v.Add("body", "{}")
	} else {
		v.Add("body", fmt.Sprintf("{gid:%d}", gid))
	}
	v.Add("_", fmt.Sprint(time.Now().Unix()))
	q := v.Encode()
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://api.m.jd.com/api?%s", q), nil)
	req.Header.Add("referer", "https://miaosha.jd.com/")
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.93 Safari/537.36")
	client := clientWithWrapper()
	res, err := client.Do(req)
	var msList MiaoshaListJson
	if err != nil {
		log.Println("秒杀请求失败")
		return msList
	}
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)
	// jsonp: `fn({});` -> `{}`
	body = body[len("pcMiaoShaAreaList")+1 : len(body)-2]
	_ = json.Unmarshal(body, &msList)
	return msList
}

// 监控秒杀信息
func SpyOnJdMiaosha(gids []uint8) {
	defer func() {
		if ok := recover(); ok != nil {
			log.Println("recover from schedule job:", ok)
		}
	}()

	groupSku := []Miaosha{}
	gidData := map[uint8]MiaoshaListJson{}
	for _, g := range gids {
		// 重复
		// 重复原因 10:00 请求 8点结束点数据, 会返回 10点数据
		if _, ok := gidData[g]; ok {
			continue
		}
		miaosha := GetMiaoshaList(g)
		// 出错
		if len(miaosha.MiaoShaList) == 0 {
			continue
		}
		gid64, _ := strconv.ParseInt(miaosha.Gid, 10, 8)
		if _, ok := gidData[uint8(gid64)]; ok {
			continue
		}
		// 重复
		// 1 重复原因 10:00 请求 8点结束点数据, 会返回 10点数据
		// 2 11:00 请求 23：00 还没开始, 返回 11 点
		gidData[uint8(gid64)] = miaosha
		goodsList := FilterGoods(miaosha.MiaoShaList, 15, 0.2)
		groupSku = append(groupSku, goodsList...)

		time.Sleep(1 * time.Second)
	}
	if len(groupSku) == 0 {
		log.Println("没有找到合适的商品")
		return
	}
	apiModel := ApiModel{authInfo.Token, TG_API, "sendMessage"}
	text := "兄弟们,冲优惠2折和15元以下商品\n"
	for _, item := range groupSku {
		// markdown 转译. \., golang 转译 \\.
		itemUrl := fmt.Sprintf("item\\.jd\\.com/%s\\.html", item.WareId)
		escapedShortName := TGSpecialChartPairsPlacer.Replace(item.ShortWname)
		escapedPrice := TGSpecialChartPairsPlacer.Replace(item.MiaoShaPrice)
		// [18:00]xxx商品-价格-sku
		text += fmt.Sprintf("[\\[%s\\-%s元\\-%s\\]%s](%s)\n", item.StartTimeShow, escapedPrice, item.WareId, escapedShortName, itemUrl)
	}
	sendTgMessage(apiModel, text, authInfo.ChatId)
}
