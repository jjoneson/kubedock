package server

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"k8s.io/klog"

	"github.com/joyrex2001/kubedock/internal/backend"
	"github.com/joyrex2001/kubedock/internal/server/httputil"
	"github.com/joyrex2001/kubedock/internal/server/routes"
)

// Server is the API server.
type Server struct {
	kub backend.Backend
}

// New will instantiate a Server object.
func New(kub backend.Backend) *Server {
	return &Server{kub: kub}
}

// Run will initialize the http api server and configure all available
// routers.
func (s *Server) Run() error {
	if !klog.V(2) {
		gin.SetMode(gin.ReleaseMode)
	}

	router := s.getGinEngine()
	router.SetTrustedProxies(nil)

	socket := viper.GetString("server.socket")
	if socket == "" {
		port := viper.GetString("server.listen-addr")
		klog.Infof("api server started listening on %s", port)
		if viper.GetBool("server.tls-enable") {
			cert := viper.GetString("server.tls-cert-file")
			key := viper.GetString("server.tls-key-file")
			router.RunTLS(port, cert, key)
		} else {
			router.Run(port)
		}
	} else {
		klog.Infof("api server started listening on %s", socket)
		router.RunUnix(socket)
	}

	return nil
}

// getGinEngine will return a gin.Engine router and configure the
// appropriate middleware.
func (s *Server) getGinEngine() *gin.Engine {
	router := gin.New()
	router.Use(httputil.VersionAliasMiddleware(router))
	router.Use(gin.Logger())
	router.Use(httputil.RequestLoggerMiddleware())
	router.Use(httputil.ResponseLoggerMiddleware())
	router.Use(gin.Recovery())

	insp := viper.GetBool("registry.inspector")
	if insp {
		klog.Infof("image inspector enabled")
	}

	pfwrd := viper.GetBool("port-forward")
	if pfwrd {
		klog.Infof("port-forwarding services to 127.0.0.1")
	}

	revprox := viper.GetBool("reverse-proxy")
	if revprox && !pfwrd {
		klog.Infof("enabled reverse-proxy services via 0.0.0.0 on the kubedock host")
	}
	if revprox && pfwrd {
		klog.Infof("ignored reverse-proxy as port-forward is enabled")
		revprox = false
	}

	prea := viper.GetBool("pre-archive")
	if prea {
		klog.Infof("copying archives without starting containers enabled")
	}

	djob := viper.GetBool("deploy-as-job")
	if djob {
		klog.Infof("use k8s jobs for container deployments enabled")
	}

	reqcpu := viper.GetString("kubernetes.request-cpu")
	if reqcpu != "" {
		klog.Infof("default cpu request: %s", reqcpu)
	}
	reqmem := viper.GetString("kubernetes.request-memory")
	if reqmem != "" {
		klog.Infof("default memory request: %s", reqmem)
	}

	runasuid := viper.GetString("kubernetes.runas-user")
	if runasuid != "" {
		klog.Infof("default runas user: %s", runasuid)
	}

	pulpol := viper.GetString("kubernetes.pull-policy")
	klog.Infof("default image pull policy: %s", pulpol)

	klog.Infof("using namespace: %s", viper.GetString("kubernetes.namespace"))

	routes.New(router, s.kub, routes.Config{
		Inspector:     insp,
		RequestCPU:    reqcpu,
		RequestMemory: reqmem,
		RunasUser:     runasuid,
		PullPolicy:    pulpol,
		PortForward:   pfwrd,
		ReverseProxy:  revprox,
		PreArchive:    prea,
		DeployAsJob:   djob,
	})

	return router
}
