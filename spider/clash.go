package spider

import (
	"github.com/metacubex/mihomo/config"
	"github.com/metacubex/mihomo/log"
	"sync"
)

func init() {
	Register(CollectClash, NewClashCollect)
}

type Clash struct {
	Url string
}

func (c *Clash) Get() []map[string]any {
	proxies := make([]map[string]any, 0)

	all := GetBytes(c.Url)
	if all != nil {
		rawCfg, err := config.UnmarshalRawConfig(all)
		if err == nil && rawCfg.Proxy != nil {
			proxies = rawCfg.Proxy
		}
	}

	return proxies
}

func (c *Clash) Get2ChanWG(pc chan []map[string]any, wg *sync.WaitGroup) {
	defer wg.Done()
	nodes := c.Get()
	log.Infoln("STATISTIC: Clash count=%d url=%s", len(nodes), c.Url)
	if len(nodes) > 0 {
		pc <- nodes
	}
}

func NewClashCollect(getter Getter) Collect {
	return &Clash{Url: getter.Url}
}
