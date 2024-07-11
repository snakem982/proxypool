package spider

import (
	"github.com/metacubex/mihomo/common/convert"
	"log"
	"regexp"
	"strings"
	"sync"
)

func init() {
	Register(CollectSharelink, NewShareLinkCollect)
}

type ShareLink struct {
	Url string
}

func (c *ShareLink) Get() []map[string]any {
	proxies := make([]map[string]any, 0)

	all := GetBytes(c.Url)
	if all != nil {
		builder := strings.Builder{}
		for _, link := range grepShareLink(all) {
			builder.WriteString(link + "\n")
		}
		if builder.Len() > 0 {
			v2ray, err := convert.ConvertsV2Ray([]byte(builder.String()))
			if err == nil && v2ray != nil {
				proxies = v2ray
			}
		}
	}

	return proxies
}

func (c *ShareLink) Get2ChanWG(pc chan []map[string]any, wg *sync.WaitGroup) {
	defer wg.Done()
	nodes := c.Get()
	log.Printf("STATISTIC: ShareLink count=%d url=%s\n", len(nodes), c.Url)
	if len(nodes) > 0 {
		pc <- nodes
	}
}

func NewShareLinkCollect(getter Getter) Collect {
	return &ShareLink{Url: getter.Url}
}

var shareLinkReg = regexp.MustCompile("(vless|vmess|trojan|ss|ssr|tuic|hysteria|hysteria2|hy2|juicity)://([A-Za-z0-9+/_&?=@:%.-])+")

// grepShareLink
//
//	@Description: 抓取分享链接
//	@param all
//	@return []string
func grepShareLink(all []byte) []string {
	return shareLinkReg.FindAllString(string(all), -1)
}
