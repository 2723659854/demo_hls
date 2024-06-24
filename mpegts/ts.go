package mpegts

import (
	"net/url"
	"strconv"

	"github.com/penndev/rtmp-go/rtmp"
)

var defaultH264HZ = 90
/* 生成ts文件 */
func Adapterts(topic string, ch <-chan rtmp.Pack) {
    /* ts文件名称 */
	filename := "runtime/" + url.QueryEscape(topic) + ".ts"
    /* 实例化一个新的ts对象 */
	t := &TsPack{}
	/* 使用文件名创建一个ts文件 */
	t.NewTs(filename)
	/* 收尾函数，删除缓存和频道 */
	defer delete(cache, topic)
	/* ts长度 整型 */
	var tslen uint32 // single tsfile sum(dts)
	/* 遍历rtmp队列的数据包 */
	for pk := range ch {
	    /* 如果长度大于5000，从代码中可以推断是1毫秒一个数据包 */
		// gen new ts file (dts 5*second)
		if tslen > 5000 {
		    /* 设置缓存中数据 */
			var extinf = ExtInf{
				Inf:  tslen,
				File: filename,
			}
			/* 如果有该频道的数据，将数据追加到缓存中 */
			// file add the hls cache
			if v, ok := cache[topic]; ok {
				cache[topic] = append(v, extinf)
			} else {
			    /* 没有就初始化这个频道  */
				cache[topic] = []ExtInf{extinf}
			}
			/* 以频道名称 + dts作为文件作为ts文件名称 */
			filename = "runtime/" + url.QueryEscape(topic) + strconv.Itoa(int(t.DTS)) + ".ts"
			/* 写入ts头 */
			t.NewTs(filename)
			/* 初始化ts长度为0 */
			tslen = 0
		}
        /* 将数据写入到ts文件，rtmp数据包类型（音频，视频），时间戳，扩展时间戳，rtmp包的载荷 */
		t.FlvTag(pk.MessageTypeID, pk.Timestamp, byte(pk.ExtendTimestamp), pk.PayLoad)
        /* 更新计数器 */
		tslen += pk.Timestamp
	}
}
