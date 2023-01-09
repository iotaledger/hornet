package restapi

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type DynamicProxy struct {
	group    *echo.Group
	balancer *balancer
}

type balancer struct {
	mutex   sync.RWMutex
	prefix  string
	targets map[string]*middleware.ProxyTarget
}

func (b *balancer) AddTarget(target *middleware.ProxyTarget) bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.targets[target.Name] = target

	return false
}

func (b *balancer) RemoveTarget(prefix string) bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	delete(b.targets, prefix)

	return true
}

func (b *balancer) Next(c echo.Context) *middleware.ProxyTarget {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	uri := b.uriFromRequest(c)

	name := strings.TrimPrefix(uri, "/")
	for k, v := range b.targets {
		if strings.HasPrefix(name, k) {
			return v
		}
	}

	return nil
}

func (b *balancer) uriFromRequest(c echo.Context) string {
	req := c.Request()
	rawURI := req.RequestURI
	if rawURI != "" && rawURI[0] != '/' {
		prefix := ""
		if req.URL.Scheme != "" {
			prefix = req.URL.Scheme + "://"
		}
		if req.URL.Host != "" {
			prefix += req.URL.Host // host or host:port
		}
		if prefix != "" {
			rawURI = strings.TrimPrefix(rawURI, prefix)
		}
	}
	rawURI = strings.TrimPrefix(rawURI, b.prefix)

	return rawURI
}

func (b *balancer) skipper(c echo.Context) bool {
	return b.Next(c) == nil
}

func (b *balancer) AddTargetHostAndPort(prefix string, host string, port uint32) error {
	apiURL, err := url.Parse(fmt.Sprintf("http://%s:%d", host, port))
	if err != nil {
		return err
	}
	b.AddTarget(&middleware.ProxyTarget{
		Name: prefix,
		URL:  apiURL,
	})

	return nil
}

func NewDynamicProxy(e *echo.Echo, prefix string) *DynamicProxy {
	balancer := &balancer{
		prefix:  prefix,
		targets: map[string]*middleware.ProxyTarget{},
	}

	proxy := &DynamicProxy{
		group:    e.Group(prefix),
		balancer: balancer,
	}

	return proxy
}

func (p *DynamicProxy) middleware(prefix string) echo.MiddlewareFunc {
	config := middleware.DefaultProxyConfig
	config.Skipper = p.balancer.skipper
	config.Balancer = p.balancer
	config.Rewrite = map[string]string{
		fmt.Sprintf("^%s/%s/*", p.balancer.prefix, prefix): "/$1",
	}

	return middleware.ProxyWithConfig(config)
}

func (p *DynamicProxy) AddGroup(prefix string) *echo.Group {
	return p.group.Group("/" + prefix)
}

func (p *DynamicProxy) AddReverseProxy(prefix string, host string, port uint32) error {
	if err := p.balancer.AddTargetHostAndPort(prefix, host, port); err != nil {
		return err
	}
	p.AddGroup(prefix).Use(p.middleware(prefix))

	return nil
}

func (p *DynamicProxy) RemoveReverseProxy(prefix string) {
	p.balancer.RemoveTarget(prefix)
}
