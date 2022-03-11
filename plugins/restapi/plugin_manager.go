package restapi

import (
	"sync"

	"github.com/labstack/echo/v4"

	restapipkg "github.com/gohornet/hornet/pkg/restapi"
)

type RestPluginManager struct {
	sync.RWMutex
	plugins []string
	proxy   *restapipkg.DynamicProxy
}

func newRestPluginManager(e *echo.Echo) *RestPluginManager {
	return &RestPluginManager{
		plugins: []string{},
		proxy:   restapipkg.NewDynamicProxy(deps.Echo, "/api/plugins"),
	}
}
func (p *RestPluginManager) Plugins() []string {
	p.RLock()
	defer p.RUnlock()

	return p.plugins
}

// AddPlugin adds a plugin route to the RouteInfo endpoint and returns the route for this plugin.
func (p *RestPluginManager) AddPlugin(pluginRoute string) *echo.Group {
	p.Lock()
	defer p.Unlock()

	found := false
	for _, plugin := range p.plugins {
		if plugin == pluginRoute {
			found = true
			break
		}
	}
	if !found {
		p.plugins = append(p.plugins, pluginRoute)
	}
	return p.proxy.AddGroup(pluginRoute)
}

// AddPluginProxy adds a plugin route to the RouteInfo endpoint and configures a remote proxy for this route.
func (p *RestPluginManager) AddPluginProxy(pluginRoute string, host string, port uint32) {
	p.Lock()
	defer p.Unlock()

	found := false
	for _, plugin := range p.plugins {
		if plugin == pluginRoute {
			found = true
			break
		}
	}
	if !found {
		p.plugins = append(p.plugins, pluginRoute)
	}
	p.proxy.AddReverseProxy(pluginRoute, host, port)
}

// RemovePlugin removes a plugin route to the RouteInfo endpoint.
func (p *RestPluginManager) RemovePlugin(pluginRoute string) {
	p.Lock()
	defer p.Unlock()

	newPlugins := make([]string, 0)
	for _, plugin := range p.plugins {
		if plugin != pluginRoute {
			newPlugins = append(newPlugins, plugin)
		}
	}
	p.plugins = newPlugins
	p.proxy.RemoveReverseProxy(pluginRoute)
}
