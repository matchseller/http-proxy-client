package util

import (
	"os"
)

//判断环境变量是否存在
func CheckWorkDir() {
	if os.Getenv("PROXY_CLIENT_WORK_DIR") == "" {
		panic("请先设置PROXY_CLIENT_WORK_DIR环境变量")
	}
}

// 判断所给路径文件/文件夹是否存在
func Exists(path string) bool {
	_, err := os.Stat(path) //os.Stat获取文件信息
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

func StringInArray(arr []string, str string) bool {
	for _, v := range arr {
		if v == str {
			return true
		}
	}
	return false
}
