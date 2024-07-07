package spider

import (
	"github.com/metacubex/mihomo/common/convert"
	"github.com/metacubex/mihomo/config"
	"github.com/metacubex/mihomo/log"
	"regexp"
	"strings"
	"sync"
	"time"
)

func init() {
	Register(CollectFuzzy, NewFuzzyCollect)
}

type Fuzzy struct {
	Url string
}

func (c *Fuzzy) Get() []map[string]any {
	content := GetBytes(c.Url)
	return ComputeFuzzy(content)
}

func (c *Fuzzy) Get2ChanWG(pc chan []map[string]any, wg *sync.WaitGroup) {
	defer wg.Done()
	nodes := c.Get()
	log.Infoln("STATISTIC: Fuzzy count=%d url=%s", len(nodes), c.Url)
	if len(nodes) > 0 {
		pc <- nodes
	}
}

func NewFuzzyCollect(getter Getter) Collect {
	return &Fuzzy{Url: getter.Url}
}

type void struct{}

var nullValue void

var re = regexp.MustCompile(`proxies|api|token|raw|subscribe|txt|yaml|yml|sub|uuid`)
var not = regexp.MustCompile(`svg|png|mp4|mp3|jpg|jpeg|m3u8|flv|gif|icon|ktv|mov|webcam`)
var urlRe = regexp.MustCompile("(https|http)://[-A-Za-z0-9\u4e00-\u9ea5+&@#/%?=~_!:,.;]+[-A-Za-z0-9\u4e00-\u9ea5+&@#/%=~_]")

func grepFuzzy(all []byte) map[string]void {
	set := make(map[string]void) // New empty set

	subUrls := urlRe.FindAllString(string(all), -1)
	for _, url := range subUrls {
		if !re.MatchString(url) || not.MatchString(url) {
			continue
		}
		set[url] = nullValue
	}

	return set
}

func ComputeFuzzy(content []byte) []map[string]any {

	proxies := make([]map[string]any, 0)
	if content == nil {
		return proxies
	}

	// 进行订阅抓取
	fuzzy := grepFuzzy(content)
	pool := NewTimeoutPoolWithDefaults()
	pool.WaitCount(len(fuzzy))

	var cFlag = regexp.MustCompile(`proxies|clash|yaml|yml`)
	lock := sync.Mutex{}
	for temp := range fuzzy {
		url := temp
		pool.SubmitWithTimeout(func(done chan struct{}) {
			defer func() {
				// 每个协程内部使用recover捕获可能在调用逻辑中发生的panic
				if e := recover(); e != nil {
					log.Errorln("====抓取错误====%s", e)
				}
				done <- struct{}{}
			}()

			getter := Getter{Url: url}
			var ok []map[string]any
			if cFlag.MatchString(url) {
				collect, _ := NewCollect(CollectClash, getter)
				ok = collect.Get()
			} else if strings.Contains(url, "README.md") {
				collect, _ := NewCollect(CollectSharelink, getter)
				ok = collect.Get()
			} else {
				all := GetBytes(url)
				if all == nil || len(all) < 16 {
					return
				}
				isGo := true
				rawCfg, err := config.UnmarshalRawConfig(all)
				if err == nil && rawCfg.Proxy != nil {
					ok = rawCfg.Proxy
					isGo = false
				}
				if isGo {
					v2ray, err := convert.ConvertsV2Ray(all)
					if err == nil && v2ray != nil {
						ok = v2ray
						isGo = false
					}
				}
				if isGo {
					builder := strings.Builder{}
					for _, link := range grepShareLink(all) {
						builder.WriteString(link + "\n")
					}
					if builder.Len() > 0 {
						all = []byte(builder.String())
						v2ray, err := convert.ConvertsV2Ray(all)
						if err == nil && v2ray != nil {
							ok = v2ray
						}
					}
				}
			}

			if ok != nil && len(ok) > 0 {
				lock.Lock()
				proxies = append(proxies, ok...)
				lock.Unlock()
			}
		}, time.Minute)
	}
	pool.StartAndWait()

	// 进行分享链接抓取
	builder := strings.Builder{}
	for _, link := range grepShareLink(content) {
		builder.WriteString(link + "\n")
	}
	if builder.Len() > 0 {
		content = []byte(builder.String())
		v2ray, err := convert.ConvertsV2Ray(content)
		if err != nil && v2ray != nil {
			lock.Lock()
			proxies = append(proxies, v2ray...)
			lock.Unlock()
		}
	}

	return proxies
}
