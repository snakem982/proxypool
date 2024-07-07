package spider

import (
	"errors"
	"github.com/snakem982/proxypool/tools"
	"golang.org/x/net/html"
	"strings"
	"sync"
)

type Getter struct {
	Id   string `json:"id,omitempty" yaml:"id,omitempty"`
	Type string `json:"type" yaml:"type"`
	Url  string `json:"url" yaml:"url"`
}

type Collect interface {
	Get() []map[string]any
	Get2ChanWG(pc chan []map[string]any, wg *sync.WaitGroup)
}

type collector func(getter Getter) Collect

var collectorMap = make(map[string]collector)

func Register(sourceType string, c collector) {
	collectorMap[sourceType] = c
}

var ErrorCreateNotSupported = errors.New("type not supported")

func NewCollect(sourceType string, getter Getter) (Collect, error) {
	if c, ok := collectorMap[sourceType]; ok {
		return c(getter), nil
	}

	return nil, ErrorCreateNotSupported
}

func GetBytes(url string) []byte {
	all := tools.ConcurrentHttpGet(url)
	if all != nil {
		temp := html.UnescapeString(string(all))
		temp = strings.Replace(temp, "\"HOST\"", "\"Host\"", -1)
		all = []byte(temp)
	}

	return all
}
