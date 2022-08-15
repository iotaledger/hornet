package restapi

import (
	"sync"

	"github.com/labstack/echo/v4"

	restapipkg "github.com/iotaledger/hornet/v2/pkg/restapi"
)

type RestRouteManager struct {
	sync.RWMutex
	routes []string
	proxy  *restapipkg.DynamicProxy
}

func newRestRouteManager(e *echo.Echo) *RestRouteManager {
	return &RestRouteManager{
		routes: []string{},
		proxy:  restapipkg.NewDynamicProxy(e, "/api"),
	}
}
func (p *RestRouteManager) Routes() []string {
	p.RLock()
	defer p.RUnlock()

	return p.routes
}

// AddRoute adds a route to the Routes endpoint and returns group to be used to register endpoints.
func (p *RestRouteManager) AddRoute(route string) *echo.Group {
	p.Lock()
	defer p.Unlock()

	found := false
	for _, r := range p.routes {
		if r == route {
			found = true

			break
		}
	}
	if !found {
		p.routes = append(p.routes, route)
	}

	// existing groups get overwritten (necessary if last plugin was not cleaned up properly)
	return p.proxy.AddGroup(route)
}

// AddProxyRoute adds a proxy route to the Routes endpoint and configures a remote proxy for this route.
func (p *RestRouteManager) AddProxyRoute(route string, host string, port uint32) error {
	p.Lock()
	defer p.Unlock()

	found := false
	for _, r := range p.routes {
		if r == route {
			found = true

			break
		}
	}
	if !found {
		p.routes = append(p.routes, route)
	}

	// existing proxies get overwritten (necessary if last plugin was not cleaned up properly)
	return p.proxy.AddReverseProxy(route, host, port)
}

// RemoveRoute removes a route from the Routes endpoint.
func (p *RestRouteManager) RemoveRoute(route string) {
	p.Lock()
	defer p.Unlock()

	newRoutes := make([]string, 0)
	for _, r := range p.routes {
		if r != route {
			newRoutes = append(newRoutes, r)
		}
	}
	p.routes = newRoutes
	p.proxy.RemoveReverseProxy(route)
}
