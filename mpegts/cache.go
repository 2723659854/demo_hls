package mpegts
/* 缓存中存储的数据的结构 */
type ExtInf struct {
    /* rtmp包计数器 */
	Inf  uint32
	/* ts文件名称 */
	File string
}
/* 缓存类 */
var cache map[string][]ExtInf
/* * 初始化一个缓存 */
func init() {
	cache = make(map[string][]ExtInf)
}
/* 开启hls服务 */
func HlsLive(topic string) ([]ExtInf, int, bool) {
    /* 从缓存中获取数据 */
	if v, ok := cache[topic]; ok {
	    /* 计算长度 */
		l := len(v)
		/* 如果数据长度小于3 */
		if l < 3 {
		    /* 那么全部返回 */
			return v, 0, ok
		} else {
		    /* 只取最新的三个数据 */
			return v[l-3:], l, ok
		}

	}
	/* 缓存中没有对应的数据 */
	return nil, 0, false
}
