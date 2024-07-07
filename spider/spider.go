package spider

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/ip2location/ip2location-go/v9"
	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/common/utils"
	"github.com/metacubex/mihomo/config"
	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/log"
	"github.com/snakem982/proxypool/tools"
	"gopkg.in/yaml.v3"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	CollectClash     = "clash"
	CollectV2ray     = "v2ray"
	CollectSharelink = "share"
	CollectFuzzy     = "fuzzy"
)

//go:embed IP2LOCATION-LITE-DB1.BIN
var fsIp2 []byte

var db *ip2location.DB

var emojiMap = make(map[string]string)

//go:embed flags.json
var fsEmoji []byte

type dBReader struct {
	reader *bytes.Reader
}

func (d dBReader) Read(p []byte) (n int, err error) {
	return d.reader.Read(p)
}

func (d dBReader) Close() error {
	return nil
}

func (d dBReader) ReadAt(p []byte, off int64) (n int, err error) {
	return d.reader.ReadAt(p, off)
}

func init() {
	db, _ = ip2location.OpenDBWithReader(dBReader{reader: bytes.NewReader(fsIp2)})

	type countryEmoji struct {
		Code  string `json:"code"`
		Emoji string `json:"emoji"`
	}
	var countryEmojiList = make([]countryEmoji, 0)
	_ = json.Unmarshal(fsEmoji, &countryEmojiList)
	for _, i := range countryEmojiList {
		emojiMap[i.Code] = i.Emoji
	}
}

func Crawl() bool {
	// 加载默认配置中的节点
	defaultBuf, defaultErr := os.ReadFile(filepath.Join(C.Path.HomeDir(), "uploads/0.yaml"))
	proxies := make([]map[string]any, 0)
	if defaultErr == nil && len(defaultBuf) > 0 {
		rawCfg, err := config.UnmarshalRawConfig(defaultBuf)
		if err == nil && len(rawCfg.Proxy) > 0 {
			proxies = rawCfg.Proxy
			log.Infoln("load default config proxies success %d", len(rawCfg.Proxy))
		}
	}

	// 获取getters
	getters := make([]Getter, 0)
	//values := cache.GetList(constant.PrefixGetter)
	//if len(values) > 0 {
	//	for _, value := range values {
	//		getter := Getter{}
	//		_ = json.Unmarshal(value, &getter)
	//		getters = append(getters, getter)
	//	}
	//}

	// 进行抓取
	if len(getters) > 0 {
		wg := &sync.WaitGroup{}
		var pc = make(chan []map[string]any)
		for _, g := range getters {
			collect, err := NewCollect(g.Type, g)
			if err != nil {
				continue
			}
			wg.Add(1)
			go collect.Get2ChanWG(pc, wg)
		}
		go func() {
			wg.Wait()
			close(pc)
		}()
		for p := range pc {
			if p != nil {
				proxies = append(proxies, p...)
			}
		}
	}

	// 去重
	maps := Unique(proxies, true)
	if len(maps) == 0 {
		return false
	}

	// 转换
	nodes := map2proxies(maps)
	if len(nodes) == 0 {
		return false
	}

	// url测速
	keys := urlTest(nodes)
	if len(keys) == 0 {
		return false
	}

	// 国家代码查询
	proxies = GetCountryName(keys, maps)

	// 排序添加emoji
	SortAddEmoji(proxies)

	if len(proxies) > 255 {
		proxies = proxies[0:256]
	}

	// 存盘
	data := make(map[string]any)
	data["proxies"] = proxies
	all, _ := yaml.Marshal(data)
	filePath := C.Path.HomeDir() + "/uploads/0.yaml"
	_ = os.Remove(filePath)
	_ = os.WriteFile(filePath, all, 0777)

	return true
}

func Unique(mappings []map[string]any, needTls bool) (maps map[string]map[string]any) {

	maps = make(map[string]map[string]any)

	for _, mapping := range mappings {
		proxyType, existType := mapping["type"].(string)
		if !existType {
			continue
		}

		var (
			proxyId string
			err     error
		)
		server := mapping["server"]
		port := mapping["port"]
		password := mapping["password"]
		uuid := mapping["uuid"]
		switch proxyType {
		case "ss":
			proxyId = fmt.Sprintf("%s|%v|%v|%v", "ss", server, port, password)
		case "ssr":
			proxyId = fmt.Sprintf("%s|%v|%v|%v", "ssr", server, port, password)
		case "vmess":
			if needTls {
				tls, existTls := mapping["tls"].(bool)
				if !existTls || !tls {
					continue
				}
			}
			proxyId = fmt.Sprintf("%s|%v|%v|%v", "vmess", server, port, uuid)
		case "vless":
			if needTls {
				tls, existTls := mapping["tls"].(bool)
				if !existTls || !tls {
					continue
				}
			}
			flow, existFlow := mapping["flow"].(string)
			if existFlow && flow != "" && flow != "xtls-rprx-vision" {
				continue
			}
			proxyId = fmt.Sprintf("%s|%v|%v|%v", "vless", server, port, uuid)
		case "trojan":
			if needTls {
				_, existSni := mapping["sni"].(string)
				if !existSni {
					continue
				}
			}
			proxyId = fmt.Sprintf("%s|%v|%v|%v", "trojan", server, port, password)
		case "hysteria":
			authStr, exist := mapping["auth_str"]
			if !exist {
				authStr = mapping["auth-str"]
			}
			proxyId = fmt.Sprintf("%s|%v|%v|%v", "hysteria", server, port, authStr)
		case "hysteria2":
			proxyId = fmt.Sprintf("%s|%v|%v|%v", "hysteria2", server, port, password)
		case "wireguard":
			authStr := mapping["private-key"]
			proxyId = fmt.Sprintf("%s|%v|%v|%v", "wireguard", server, port, authStr)
		case "tuic":
			proxyId = fmt.Sprintf("%s|%v|%v|%v|%v", "tuic", server, port, uuid, password)
		default:
			err = fmt.Errorf("unsupport proxy type: %s", proxyType)
		}

		if err != nil {
			continue
		}
		temp := mapping
		temp["name"] = proxyId
		maps[proxyId] = temp
	}

	return
}

func map2proxies(maps map[string]map[string]any) (proxies []C.Proxy) {
	pool := NewTimeoutPoolWithDefaults()
	pool.WaitCount(len(maps))
	mutex := sync.Mutex{}

	proxies = make([]C.Proxy, 0)
	for _, m := range maps {
		proxy := m
		pool.SubmitWithTimeout(func(done chan struct{}) {
			defer func() {
				if e := recover(); e != nil {
					log.Errorln("===map2proxies===%s", e)
				}
				done <- struct{}{}
			}()
			proxyT, err := adapter.ParseProxy(proxy)
			if err == nil {
				mutex.Lock()
				proxies = append(proxies, proxyT)
				mutex.Unlock()
			}
		}, 2*time.Second)
	}
	pool.StartAndWait()

	return
}

func urlTest(proxies []C.Proxy) []string {
	pool := NewTimeoutPoolWithDefaults()
	keys := make([]string, 0)
	m := sync.Mutex{}

	expectedStatus, _ := utils.NewUnsignedRanges[uint16]("200/204/301/302")
	url := "https://www.gstatic.com/generate_204"

	pool.WaitCount(len(proxies))
	for _, p := range proxies {
		proxy := p
		pool.SubmitWithTimeout(func(done chan struct{}) {
			defer func() {
				if e := recover(); e != nil {
					log.Errorln("===urlTest===%s", e)
				}
				done <- struct{}{}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*4500)
			defer cancel()
			_, err := proxy.URLTest(ctx, url, expectedStatus)
			if err == nil {
				m.Lock()
				keys = append(keys, proxy.Name())
				m.Unlock()
			}
		}, 5*time.Second)
	}
	pool.StartAndWait()

	return keys
}

func GetCountryName(keys []string, maps map[string]map[string]any) []map[string]any {

	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 100 * time.Millisecond,
			}
			return d.DialContext(ctx, network, "1.1.1.1")
		},
	}

	proxies := make([]map[string]any, 0)
	ipLock := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(len(keys))
	for _, key := range keys {
		m := maps[key]
		m["name"] = "ZZ"
		go func() {
			defer wg.Done()

			ipOrDomain := m["server"].(string)
			if tools.CheckStringAlphabet(ipOrDomain) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
				defer cancel()
				iPv4, err := r.LookupIP(ctx, "ip4", ipOrDomain)
				if err == nil && iPv4 != nil {
					m["name"] = getCountryCode(iPv4[0].String())
				}
			} else {
				m["name"] = getCountryCode(ipOrDomain)
			}
			ipLock.Lock()
			proxies = append(proxies, m)
			ipLock.Unlock()
		}()
	}

	wg.Wait()
	return proxies
}

func getCountryCode(domain string) string {
	countryCode := "ZZ"
	record, err := db.Get_country_short(domain)
	if err != nil || len(record.Country_short) != 2 {
		return countryCode
	}

	return record.Country_short
}

func getIndex(at string) int {
	switch at {
	case "hysteria2":
		return 1
	case "hysteria":
		return 2
	case "tuic":
		return 3
	case "ss":
		return 5
	case "vless":
		return 6
	default:
		return 10
	}
}

func SortAddEmoji(proxies []map[string]any) {
	sort.Slice(proxies, func(i, j int) bool {
		iProtocol := proxies[i]["type"].(string)
		jProtocol := proxies[j]["type"].(string)

		if getIndex(iProtocol) != getIndex(jProtocol) {
			return getIndex(iProtocol) < getIndex(jProtocol)
		}

		if proxies[i]["name"].(string) != proxies[j]["name"].(string) {
			return proxies[i]["name"].(string) < proxies[j]["name"].(string)
		}

		return tools.Reverse(proxies[i]["server"].(string)) < tools.Reverse(proxies[j]["server"].(string))
	})

	for i, _ := range proxies {
		name := proxies[i]["name"].(string)
		name = fmt.Sprintf("%s %s_%+02v", emojiMap[name], name, i+1)
		proxies[i]["name"] = strings.TrimSpace(name)
	}
}

func SortAddIndex(proxies []map[string]any) []map[string]any {
	// 去重
	maps := Unique(proxies, false)
	keys := make([]string, 0)
	for k := range maps {
		keys = append(keys, k)
	}
	// 国家代码查询
	proxies = GetCountryName(keys, maps)
	// 排序添加emoji
	SortAddEmoji(proxies)

	return proxies
}
