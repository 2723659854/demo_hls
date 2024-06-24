package mpegts

import (
	"encoding/binary"
	"io"
	"log"
	"os"
)

type TsPack struct {
	VideoContinuty byte
	AudioContinuty byte
	DTS            uint32
	IDR            []byte
	w              io.Writer
}
/* ts头 */
func (t *TsPack) toHead(adapta, mixed bool, mtype byte) []byte {
    /* 生成长度为4的数组  */
	tsHead := make([]byte, 4)
	/* 第一位是 0x47 */
	tsHead[0] = 0x47
	if adapta {
		tsHead[1] |= 0x40
	}
	/* 如果是视频 */
	if mtype == VideoMark {
		tsHead[1] |= 1
		tsHead[2] |= 0
		/* 视频帧的计数器 */
		tsHead[3] |= t.VideoContinuty
		/* 更新视频帧的计数器 */
		t.VideoContinuty = (t.VideoContinuty + 1) % 16
		// log.Println(t.VideoContinuty)
	} else if mtype == AudioMark {
	    /* 如果是音频帧  */
		tsHead[1] |= 1
		tsHead[2] |= 1
		tsHead[3] |= t.AudioContinuty
		/* 更新音频帧的计数器 */
		t.AudioContinuty = (t.AudioContinuty + 1) % 16
	}
	if adapta || mixed {
		tsHead[3] |= 0x30
	} else {
		tsHead[3] |= 0x10
	}
	return tsHead
}

/* 将pes数据打包成ts文件 */
func (t *TsPack) toPack(mtype byte, pes []byte) {
    /* 初始化两个变量  */
	adapta := true
	mixed := false
	/* 看着像是个死循环 */
	for {
	    /* 计算pes的长度 */
		pesLen := len(pes)
		/* 如果长度小于0 ，退出任务 */
		if pesLen <= 0 {
			break
		}
		/* 如果长度小于184，刚好打一个包 */
		if pesLen < 184 {
			mixed = true
		}
		/* 初始化一个188个字节的包 */
		cPack := make([]byte, 188)
		/* 先填充为 0xff */
		for i := range cPack {
			cPack[i] = 0xff
		}
		/* 前4位是header */
		copy(cPack[0:4], t.toHead(adapta, mixed, mtype))
		/* 刚好一个小包，不用分割成多个ts文件 */
		if mixed {
		    /* 需要填充的字节数 */
			fillLen := 183 - pesLen
			/* 第4位写入需要填充的长度 */
			cPack[4] = byte(fillLen)
			/* 如果需要填充字节数大于0 */
			if fillLen > 0 {
			    /* 第5位变更为0 */
				cPack[5] = 0
			}
			/* 将pes所有内容，复制到cpack包的非填充位 */
			copy(cPack[fillLen+5:188], pes[:pesLen])
            /* 清空切片 */
			pes = pes[pesLen:]
		} else if adapta {
		    /* 长度大于了184，需要分割成多个ts数据包 */
			// 获取pcr 第4位变更为 7
			cPack[4] = 7
			/* 将计算后的pcr写入到包的5-12位 */
			copy(cPack[5:12], hexPcr(t.DTS*uint32(defaultH264HZ)))
			/* 将pes的前176个字节复制给cpack */
			copy(cPack[12:188], pes[0:176])
			/* 更新pes包内容，删除已被写入的数据 */
			pes = pes[176:]
		} else {
		    /* 分包，将pes的前184位写入到cpack 第4-188位 */
			copy(cPack[4:188], pes[0:184])
			/* 更新pes包内容 */
			pes = pes[184:]
		}
		/* 写入到ts文件中 */
		adapta = false
		t.w.Write(cPack)
	}
}
/* 视频帧 */
func (t *TsPack) videoTag(tagData []byte) {
    /* 获取编码方式 */
	codecID := tagData[0] & 0x0f
	if codecID != 7 {
		log.Println("遇到了不是h264的视频数据", codecID)
	}
    /* 校正时间戳 */
	compositionTime := binary.BigEndian.Uint32([]byte{0, tagData[2], tagData[3], tagData[4]})
	/* 初始化nalu数据 */
	nalu := []byte{}
	/* 解码帧 */
	if tagData[1] == 0 { //avc IDR frame | flv sps pps
	    /* 现将解码帧存入到内存 */
		t.IDR = tagData
		/* 计算sps的长度 */
		spsLen := int(binary.BigEndian.Uint16(tagData[11:13]))
		/* 获取sps数据 */
		sps := tagData[13 : 13+spsLen]
		/* 组装解码数据 */
		spsnalu := append([]byte{0, 0, 0, 1}, sps...)
		/* 将解码头写入到nalu 从代码中看，解码头在前面 */
		nalu = append(nalu, spsnalu...)
		/* 获取pps */
		ppsLen := int(binary.BigEndian.Uint16(tagData[14+spsLen : 16+spsLen]))
		/* 获取pps的数据 */
		pps := tagData[16+spsLen : 16+spsLen+ppsLen]
		/* 组装pps */
		ppsnalu := append([]byte{0, 0, 0, 1}, pps...)
		/* 将pps追加到nalu */
		nalu = append(nalu, ppsnalu...)
	} else if tagData[1] == 1 { //avc nalu
		/* 数据帧 首先跳过5个字节*/
		readed := 5
		/* 原始数据大于已读字节数 */
		for len(tagData) > (readed + 5) {
		    /* 一次读4个字节  */
			readleng := int(binary.BigEndian.Uint32(tagData[readed : readed+4]))
			/* 更新已读数据长度 */
			readed += 4
			/* 先追加一个头 */
			nalu = append(nalu, []byte{0, 0, 0, 1}...)
			/* 读取数据并追加到nalu */
			nalu = append(nalu, tagData[readed:readed+readleng]...)
			/* 更新指针 */
			readed += readleng
		}
	} //else panic
	/* 解码时间戳 */
	dts := t.DTS * uint32(defaultH264HZ)
	/* 播放时间戳 = 解码时间戳 + 校验时间戳 */
	pts := dts + compositionTime*uint32(defaultH264HZ)
	/* 生成pes */
	pes := PES(VideoMark, pts, dts)
	/* 打包成ts文件 视频帧的数据 是pes在前，nalu在后 */
	t.toPack(VideoMark, append(pes, nalu...))
}
/* 将音频数据追加到ts文件 */
func (t *TsPack) audioTag(tagData []byte) {
    /* 判断数据格式 */
	soundFormat := (tagData[0] & 0xf0) >> 4
	if soundFormat != 10 {
		log.Println("遇到了不是aac的音频数据")
	}
	/* aac数据的raw数据 */
	if tagData[1] == 1 {
	    /* 数据是从三位开始的  */
		tagData = tagData[2:]
		/* adtsheader  解码信息 */
		adtsHeader := []byte{0xff, 0xf1, 0x4c, 0x80, 0x00, 0x00, 0xfc}
		/* 解码头长度 */
		adtsLen := uint16(((len(tagData) + 7) << 5) | 0x1f)
		/* 将长度写入到解码头的第4-6位 */
		binary.BigEndian.PutUint16(adtsHeader[4:6], adtsLen)
		/* 拼接解码头和实际的数据adts = adtsHeader + tagData */
		adts := append(adtsHeader, tagData...)
		/* 播放时间戳 = 解码时间戳 * 90 */
		pts := t.DTS * uint32(defaultH264HZ)
		/* 打包成pes */
		pes := PES(AudioMark, pts, 0)
		/* 将pes打包，我勒个擦额，pes包的结构是 音视频原始数据 + 解码adts */

		t.toPack(AudioMark, append(pes, adts...))
	}

}
/* 追加音视频数据到ts文件 是视频帧类型 时间戳 扩展时间戳 rtmp数据载荷 */
func (t *TsPack) FlvTag(tagType byte, timeStreamp uint32, timeStreampExtended byte, tagData []byte) {
	// 组合视频戳
	/* 组合解码时间戳dts */
	dts := uint32(timeStreampExtended)*16777216 + timeStreamp
	t.DTS += dts
    /* 如果是视频帧 */
	if tagType == 9 {
		t.videoTag(tagData)
	} else if tagType == 8 {
	    /* 如果是音频帧 */
		t.audioTag(tagData)
	}
}

/* 写入ts头 */
func (t *TsPack) NewTs(filename string) {
	var err error
	/* 打开ts文件 */
	if t.w, err = os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0644); err != nil {
		log.Println(err)
	}
	/* 写入描述信息 */
	t.w.Write(SDT())
	/* 写入pat */
	t.w.Write(PAT())
	/* 写入pmt */
	t.w.Write(PMT())
	/* 将视频解码帧写入到ts文件 */
	if len(t.IDR) > 0 {
	    /* 队列中的数据全是视频帧 */
		t.videoTag(t.IDR)
	}

}
