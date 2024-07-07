package tools

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// ReadConfig 从指定路径读取配置文件内容并返回字节切片
func ReadConfig(path string) ([]byte, error) {
	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}
	// 读取文件内容
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 检查文件内容是否为空
	if len(data) == 0 {
		return nil, fmt.Errorf("配置文件 %s 为空", path)
	}

	return data, err
}

// CheckStringAlphabet
//
//	@Description: 检查字符串是否为域名
//	@param str
//	@return bool
func CheckStringAlphabet(str string) bool {
	L := len(str)
	if L < 1 {
		return false
	}
	charVariable := str[L-1:]
	// ipv6
	if strings.Contains(str, ":") {
		return false
	}
	// ipv4
	if charVariable >= "0" && charVariable <= "9" {
		return false
	}

	return true
}

// Reverse
//
//	@Description: 反转域名字符串
//	@param s
//	@return string
func Reverse(s string) string {
	if !CheckStringAlphabet(s) {
		return s
	}
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}

	return string(r)
}

// GetFreePort 函数用于获取一个可用的随机端口号，并在函数结束后关闭监听器。
func GetFreePort() (int, error) {
	return GetFreeWithPort(0)
}

// GetFreeWithPort 检测端口是否可用
func GetFreeWithPort(port int) (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}

	defer func(l *net.TCPListener) {
		err := l.Close()
		if err != nil {

		}
	}(l)
	return l.Addr().(*net.TCPAddr).Port, nil
}
