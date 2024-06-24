package hls

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/penndev/rtmp-go/mpegts"
	"github.com/penndev/rtmp-go/rtmp"
)
/* 索引列表 每隔6秒生成一個切片 */
var HlsHeader = `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-ALLOW-CACHE:YES
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:<sequence>`
/* hls服務 订阅rtmp服务 */
func Handlehls(subtop func(string) (*rtmp.PubSub, bool)) func(http.ResponseWriter, *http.Request) {
/* 处理http请求  */
	return func(w http.ResponseWriter, r *http.Request) {
	/* 获取query参数 */
		param := r.URL.Query()
		/* 获取topic就是路由 */
		topic := param.Get("topic")
		/* 如果能够获取到topic */
		if _, ok := subtop(topic); ok {
		    /* 生成mpegts包 */
			if c, l, ok := mpegts.HlsLive(topic); ok {
			    /* 替换索引内容 */
				s := strings.Replace(HlsHeader, "<sequence>", strconv.Itoa(l), 1)
				/* 更新索引里面的ts节目列表 */
				for _, v := range c {
					s += "\n#EXTINF:" + strconv.Itoa(int(v.Inf/1000)) + "." + strconv.Itoa(int(v.Inf%1000)) + "\n" + v.File
				}
				/* 返回数据 */
				w.Write([]byte(s))
			} else {
			    /* 生成ts文件失败 */
				http.Error(w, "service close", 400)
			}
		} else {
		    /* 没有找到这一路直播资源 */
			http.NotFound(w, r)
		}
	}
}
