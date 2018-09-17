package consul

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gliderlabs/registrator/bridge"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-cleanhttp"
)

const (
	DefaultInterval = "10s"
	DefaultStatus   = "warning"
)

func init() {
	f := new(Factory)
	bridge.Register(f, "consul")
	bridge.Register(f, "consul-tls")
	bridge.Register(f, "consul-unix")
}

func (r *ConsulAdapter) interpolateService(script string, service *bridge.Service) string {
	withIp := strings.Replace(script, "$SERVICE_IP", service.IP, -1)
	withPort := strings.Replace(withIp, "$SERVICE_PORT", strconv.Itoa(service.Port), -1)
	return withPort
}

type Factory struct{}

func (f *Factory) New(uri *url.URL) bridge.RegistryAdapter {
	config := consulapi.DefaultConfig()

	if uri.Scheme == "consul-unix" {
		config.Address = strings.TrimPrefix(uri.String(), "consul-")
	} else if uri.Scheme == "consul-tls" {
		tlsConfigDesc := &consulapi.TLSConfig{
			Address:            uri.Host,
			CAFile:             os.Getenv("CONSUL_CACERT"),
			CertFile:           os.Getenv("CONSUL_TLSCERT"),
			KeyFile:            os.Getenv("CONSUL_TLSKEY"),
			InsecureSkipVerify: false,
		}
		tlsConfig, err := consulapi.SetupTLSConfig(tlsConfigDesc)
		if err != nil {
			log.Fatal("Cannot set up Consul TLSConfig", err)
		}
		config.Scheme = "https"
		transport := cleanhttp.DefaultPooledTransport()
		transport.TLSClientConfig = tlsConfig
		config.HttpClient.Transport = transport
		config.Address = uri.Host
	} else if uri.Host != "" {
		config.Address = uri.Host
	}
	client, err := consulapi.NewClient(config)
	if err != nil {
		log.Fatal("consul: ", uri.Scheme)
	}
	return &ConsulAdapter{client: client}
}

type ConsulAdapter struct {
	client *consulapi.Client
}

// Ping will try to connect to consul by attempting to retrieve the current leader.
func (r *ConsulAdapter) Ping() error {
	status := r.client.Status()
	leader, err := status.Leader()
	if err != nil {
		return err
	}
	log.Println("consul: current leader ", leader)

	return nil
}

func (r *ConsulAdapter) Register(service *bridge.Service) error {
	registration := new(consulapi.AgentServiceRegistration)
	registration.ID = service.ID
	registration.Name = service.Name
	registration.Port = service.Port
	registration.Tags = service.Tags
	registration.Address = service.IP
	registration.Check = r.buildCheck(service)
	return r.client.Agent().ServiceRegister(registration)
}

func (r *ConsulAdapter) buildCheck(service *bridge.Service) *consulapi.AgentServiceCheck {
	check := consulapi.AgentServiceCheck{
		CheckID:  r.getCheckAttr(service.Attrs, "id", fmt.Sprintf("service:%s", service.ID)),
		Name:     r.getCheckAttr(service.Attrs, "name", fmt.Sprintf("Check service: %s", service.Origin.ContainerName)),
		Interval: r.getCheckAttr(service.Attrs, "interval", DefaultInterval),
		Status:   r.getCheckAttr(service.Attrs, "initial_status", DefaultStatus),

		DeregisterCriticalServiceAfter: r.getCheckAttr(service.Attrs, "deregister_critical_service_after", "1h"),
	}

	if path, ok := service.Attrs["check_http"]; ok && path != "" {
		check.HTTP = fmt.Sprintf("http://%s:%d%s", service.IP, service.Port, path)
		if method := r.getCheckAttr(service.Attrs, "method", ""); method != "" {
			check.Method = method
		}
	} else if path, ok := service.Attrs["check_https"]; ok && path != "" {
		check.HTTP = fmt.Sprintf("https://%s:%d%s", service.IP, service.Port, path)
		if skipVerify, err := strconv.ParseBool(r.getCheckAttr(service.Attrs, "tls_skip_verify", "true")); err != nil {
			check.TLSSkipVerify = true
		} else {
			check.TLSSkipVerify = skipVerify
		}
		if method := r.getCheckAttr(service.Attrs, "method", ""); method != "" {
			check.Method = method
		}
	} else if args, ok := service.Attrs["check_script"]; ok && args != "" {
		check.Args = strings.Split(r.interpolateService(args, service), " ")
	} else if args, ok := service.Attrs["check_docker"]; ok && args != "" {
		check.Args = strings.Split(r.interpolateService(args, service), "")
		check.DockerContainerID = service.Origin.ContainerID
		check.Shell = r.getCheckAttr(service.Attrs, "shell", "/bin/sh")
	} else if tcp, ok := service.Attrs["check_tcp"]; ok && tcp != "" {
		if okk, err := strconv.ParseBool(tcp); err == nil && okk {
			check.TCP = fmt.Sprintf("%s:%d", service.IP, service.Port)
			if timeout := service.Attrs["check_timeout"]; timeout != "" {
				check.Timeout = timeout
			}
		}
	} else if ttl, ok := service.Attrs["check_ttl"]; ok && ttl != "" {
		check.TTL = fmt.Sprintf("%ss", strings.TrimSuffix(ttl, "s"))
		check.Interval = ""
	} else {
		return nil
	}
	return &check
}

func (r *ConsulAdapter) Deregister(service *bridge.Service) error {
	return r.client.Agent().ServiceDeregister(service.ID)
}

func (r *ConsulAdapter) Refresh(service *bridge.Service) error {
	return nil
}

func (r *ConsulAdapter) Services() ([]*bridge.Service, error) {
	services, err := r.client.Agent().Services()
	if err != nil {
		return []*bridge.Service{}, err
	}
	out := make([]*bridge.Service, len(services))
	i := 0
	for _, v := range services {
		s := &bridge.Service{
			ID:   v.ID,
			Name: v.Service,
			Port: v.Port,
			Tags: v.Tags,
			IP:   v.Address,
		}
		out[i] = s
		i++
	}
	return out, nil
}

func (r *ConsulAdapter) getCheckAttr(attrs map[string]string, key, fallback string) string {
	if value, ok := attrs[fmt.Sprintf("check_%s", strings.ToLower(key))]; value != "" && ok {
		return value
	}
	if value, ok := os.LookupEnv(strings.ToUpper(fmt.Sprintf("consul_check_%s", key))); value != "" && ok {
		return value
	}
	return fallback
}
