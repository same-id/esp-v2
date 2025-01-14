package routegen

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/esp-v2/src/go/configgenerator/clustergen"
	"github.com/GoogleCloudPlatform/esp-v2/src/go/configgenerator/routegen/helpers"
	"github.com/GoogleCloudPlatform/esp-v2/src/go/options"
	"github.com/GoogleCloudPlatform/esp-v2/src/go/util"
	"github.com/GoogleCloudPlatform/esp-v2/src/go/util/httppattern"
	routepb "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/golang/glog"
	servicepb "google.golang.org/genproto/googleapis/api/serviceconfig"
)

// HealthCheckGenerator is a RouteGenerator that creates routes for Envoy health
// checks.
type HealthCheckGenerator struct {
	HealthzPath                  string
	AutogeneratedOperationPrefix string
	ESPOperationAPI              string

	// Technically these health checks should be handled directly by Envoy and never
	// reach the backend. However, the routes configs must contain the backend
	// cluster info in order for health check to be preformed.
	LocalBackendClusterName string
	BackendRouteGen         *helpers.BackendRouteGenerator
}

// NewHealthCheckRouteGensFromOPConfig creates HealthCheckGenerator
// from OP service config + ESPv2 options.
// It is a RouteGeneratorOPFactory.
func NewHealthCheckRouteGensFromOPConfig(serviceConfig *servicepb.Service, opts options.ConfigGeneratorOptions) ([]RouteGenerator, error) {
	if opts.Healthz == "" {
		glog.Info("Not adding health check filter gen because healthz path is not specified.")
		return nil, nil
	}

	return []RouteGenerator{
		&HealthCheckGenerator{
			HealthzPath:                  opts.Healthz,
			AutogeneratedOperationPrefix: opts.HealthCheckAutogeneratedOperationPrefix,
			ESPOperationAPI:              opts.HealthCheckOperation,
			// Health check is always against local cluster.
			// Remote clusters are not supported.
			LocalBackendClusterName: clustergen.MakeLocalBackendClusterName(serviceConfig),
			BackendRouteGen:         helpers.NewBackendRouteGeneratorFromOPConfig(opts),
		},
	}, nil
}

// GenRouteConfig implements interface RouteGenerator.
//
// Forked from `service_info.go: processHttpRule()`.
func (g *HealthCheckGenerator) GenRouteConfig() ([]*routepb.Route, error) {
	healthzPath := g.HealthzPath
	if !strings.HasPrefix(healthzPath, "/") {
		healthzPath = fmt.Sprintf("/%s", healthzPath)
	}

	uriTemplate, err := httppattern.ParseUriTemplate(healthzPath)
	if err != nil {
		return nil, err
	}

	httpRule := &httppattern.Pattern{
		HttpMethod:  util.GET,
		UriTemplate: uriTemplate,
	}

	operationName := fmt.Sprintf("%s.%s_HealthCheck", g.ESPOperationAPI, g.AutogeneratedOperationPrefix)
	methodCfg := &helpers.MethodCfg{
		OperationName:      operationName,
		BackendClusterName: g.LocalBackendClusterName,
		Deadline:           util.DefaultResponseDeadline,
		HTTPPattern:        httpRule,
	}

	return g.BackendRouteGen.GenRoutesForMethod(methodCfg)
}
