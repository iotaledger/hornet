package restapi

import (
	"errors"
	"fmt"
	"net/http"
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

func (b *balancer) AddTargetHostAndPort(prefix string, host string, port uint32, path string) error {
	if path != "" && !strings.HasPrefix(path, "/") {
		return errors.New("if path is set, it needs to start with \"/\"")
	}

	apiURL, err := url.Parse(fmt.Sprintf("http://%s:%d%s", host, port, path))
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
	hasAcceptEncodingGZIP := func(header http.Header) bool {
		return strings.Contains(header.Get(echo.HeaderAcceptEncoding), "gzip")
	}

	config := middleware.DefaultProxyConfig
	config.Skipper = p.balancer.skipper
	config.Balancer = p.balancer
	config.Rewrite = map[string]string{
		fmt.Sprintf("^%s/%s/*", p.balancer.prefix, prefix): "/$1",
	}

	configUncompressed := middleware.DefaultProxyConfig
	configUncompressed.Skipper = p.balancer.skipper
	configUncompressed.Balancer = p.balancer
	configUncompressed.Rewrite = map[string]string{
		fmt.Sprintf("^%s/%s/*", p.balancer.prefix, prefix): "/$1",
	}
	//nolint:forcetypeassert // we can safely assume that the DefaultTransport is a http.Transport
	transportUncompressed := http.DefaultTransport.(*http.Transport).Clone()
	transportUncompressed.DisableCompression = true
	configUncompressed.Transport = transportUncompressed

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// we forward compressed requests if the client also supports compressed responses
			compressed := hasAcceptEncodingGZIP(c.Request().Header)

			// we need to remove "Accept-Encoding" headers in the request,
			// because the transport handles this automatically if not set
			c.Request().Header.Del(echo.HeaderAcceptEncoding)

			if !compressed {
				return middleware.ProxyWithConfig(configUncompressed)(next)(c)
			}

			return middleware.ProxyWithConfig(config)(next)(c)
		}
	}
}

func (p *DynamicProxy) AddGroup(prefix string) *echo.Group {
	return p.group.Group("/" + prefix)
}

func (p *DynamicProxy) AddReverseProxy(prefix string, host string, port uint32, path string) error {
	if err := p.balancer.AddTargetHostAndPort(prefix, host, port, path); err != nil {
		return err
	}
	p.AddGroup(prefix).Use(p.middleware(prefix))

	return nil
}

func (p *DynamicProxy) RemoveReverseProxy(prefix string) {
	p.balancer.RemoveTarget(prefix)
}
