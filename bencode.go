package dht

import (
	"errors"
	"fmt"
	"bytes"
	"sort"
	"unicode"
	"unicode/utf8"
	"strconv"
)

/**
	维基百科：https://zh.wikipedia.org/wiki/Bencode

	字符串(utf-8)：  长度:字符串
	整形：i数字e
	列表：l嵌套内容e
	字典：d嵌套内容e
*/

func encodeString(data string) (encData []byte, err error) {
	encData = []byte(fmt.Sprintf("%d:%s", len(data), data))
	return
}

func encodeInt(data int) (encData []byte, err error) {
	encData = []byte(fmt.Sprintf("i%de", data))
	return
}

func encodeList(data []interface{}) (encData []byte, err error){
	var (
		encList = [][]byte{[]byte("l")}
		encElem []byte
	)

	for _, elem := range data {
		if encElem, err = Encode(elem); err != nil {
			return
		}
		encList = append(encList, encElem)
	}
	encList = append(encList, []byte("e"))
	encData = bytes.Join(encList, []byte(""))
	return
}

func encodeDict(data map[string]interface{}) (encData []byte, err error) {
	var (
		encMap = map[string][]byte{}
		encKey []byte
		encValue []byte
	)
	for key, value := range data {
		if encValue, err = Encode(value); err != nil {
			return
		}
		encMap[key] = encValue
	}

	sortedKeys := make([]string, 0, len(encMap))
	for key, _ := range data {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	encList := [][]byte{[]byte("d")}
	for _, key := range sortedKeys {
		if encKey, err = Encode(key); err != nil {
			return
		}
		encList = append(encList, encKey, encMap[key])
	}
	encList = append(encList, []byte("e"))
	encData = bytes.Join(encList, []byte(""))
	return
}

/**
	编码函数
 */
func Encode(data interface{}) ([]byte, error) {
	switch data.(type) {
	case string:
		return encodeString(data.(string))
	case int:
		return encodeInt(data.(int))
	case []interface{}:
		return encodeList(data.([]interface{}))
	case map[string]interface{}:
		return encodeDict(data.(map[string]interface{}))
	default:
		return nil, errors.New("invalid type")
	}
}

func decodeDict(data []byte) (decData interface{}, size int, err error) {
	return
}

func decodeList(data []byte) (decData interface{}, size int, err error) {
	return
}

func decodeInt(data []byte) (decData interface{}, size int, err error) {
	var (
		value int
		endIndex int
	)
	if len(data) < 3 || data[0] != 'i' {
		goto ERROR
	}

	// 找出utf-8字符串序列中的字母e（必须使用rune,因为utf-8的字符由多字节组成,可能包含e）
	if endIndex = bytes.IndexRune(data, 'e'); endIndex == -1 {
		goto ERROR
	}

	// 解析中间部分为整形
	if value, err = strconv.Atoi(string(data[1:endIndex])); err != nil {
		goto ERROR
	}
	return value, endIndex + 1, nil
ERROR:
	return nil, 0, errors.New("invalid int")
}

func decodeString(data []byte) (decData interface{}, size int, err error) {
	var (
		value string
		valueLen int
		endIndex int
	)
	if len(data) < 2 {
		goto ERROR
	}

	// 找出utf-8字符串序列中的字母:
	if endIndex = bytes.IndexRune(data, ':'); endIndex == -1 {
		goto ERROR
	}

	// :左侧解析为字符串长度
	if valueLen, err = strconv.Atoi(string(data[:endIndex])); err != nil {
		goto ERROR
	}

	// :右侧必须有valueLen个字节, 并且是合法utf-8
	if endIndex + valueLen + 1 > len(data) {
		goto ERROR
	}

	value = string(data[endIndex + 1 : endIndex + 1 + valueLen])
	size = len(value) + 2

	// 反向校验utf-8合法性
	data = data[endIndex + 1 : endIndex + 1 + valueLen]
	for {
		if char, size := utf8.DecodeLastRune(data); char == utf8.RuneError {
			if size != 0 { // utf-8序列不合法
				goto ERROR
			} else { // 全部解析完成
				break
			}
		} else {
			valueLen -= size
			data = data[endIndex + 1 : endIndex + 1 + valueLen]
		}
	}
	return value, size, nil
ERROR:
	return nil, 0, errors.New("invalid string")
}

func decode(data []byte) (decData interface{}, size int, err error) {
	if len(data) != 0 {
		dataType, _ := utf8.DecodeRune(data)
		if dataType == 'd' {
			return decodeDict(data)
		} else if dataType == 'l' {
			return decodeList(data)
		} else if dataType == 'i' {
			return decodeInt(data)
		} else if unicode.IsDigit(dataType) {
			return decodeString(data)
		}
	}
	return nil, 0, errors.New("invalid data")
}

/**
	解码函数
*/
func Decode(data []byte) (decData interface{}, err error) {
	decData, _, err = decode(data)
	return decData, err
}

