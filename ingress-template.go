package main

//go:generate stringer -type=ingressGroup

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (hs *hostService) UnmarshalJSON(b []byte) error {
	var i interface{}
	err := json.Unmarshal(b, &i)
	if err != nil {
		return err
	}

	switch v := i.(type) {
	case string:
		*hs = hostService{Host: v}
	case map[string]interface{}:
		*hs = hostService{}

		if d, ok := v["host"]; ok {
			hs.Host = d.(string)
		}
		if d, ok := v["service-name"]; ok {
			hs.ServiceName = d.(string)
		}
		if d, ok := v["service-port"]; ok {
			hs.ServicePort = d.(int)
		}

	default:
		return fmt.Errorf("unknown type for hostService: %T", i)
	}

	return nil
}

type hostService struct {
	Host        string `json:"host"`
	ServiceName string `json:"service-name"`
	ServicePort int    `json:"service-port"`
}

type hostList []hostService

type hostGroups map[string]hostList

// Config is the data structure the user configures
type Config struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	IngressClass string `json:"ingress-class"`

	ServiceName string `json:"service-name"`
	ServicePort int    `json:"service-port"`

	Plain       hostList   `json:"plain"`
	TLSOptional hostGroups `json:"tls-optional"`
	TLSRequired hostGroups `json:"tls-required"`
}

type ingressGroup uint8

// tlsOptional and tlsRequired are ingressGroups
const (
	tlsOptional ingressGroup = iota
	tlsRequired
)

func loadConfig() (*Config, error) {
	configFile := flag.String("file", "ingress-hosts.json", "Input file")

	ingressName := flag.String("name", "", "override ingress name")
	ingressNamespace := flag.String("namespace", "", "override namespace")
	ingressClass := flag.String("ingress-class", "", "override ingress.class")

	flag.Parse()

	b, err := ioutil.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("could not read config: %s", err)
	}
	var config Config
	err = json.Unmarshal(b, &config)
	if err != nil {
		log.Fatalf("could not read config: %s", err)
	}

	if len(*ingressName) > 0 {
		config.Name = *ingressName
	}
	if len(*ingressNamespace) > 0 {
		config.Namespace = *ingressNamespace
	}
	if len(*ingressClass) > 0 {
		config.IngressClass = *ingressClass
	}

	if config.ServicePort == 0 {
		config.ServicePort = 80
	}

	return &config, nil

}

func main() {

	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %s", err)
	}

	ingressList := v1beta1.IngressList{}
	ingressList.APIVersion = "v1"
	ingressList.Kind = "List"

	for _, group := range []ingressGroup{tlsOptional, tlsRequired} {

		name := config.Name
		if group == tlsRequired {
			name = name + "-tls"
		}

		ingress := v1beta1.Ingress{
			ObjectMeta: v1.ObjectMeta{
				Namespace: config.Namespace,
				Name:      name,
			},
		}
		ingress.Kind = "Ingress"
		ingress.APIVersion = "extensions/v1beta1"
		ingress.Kind = "Ingress"
		if ingress.Annotations == nil {
			ingress.Annotations = map[string]string{}
		}
		if len(config.IngressClass) > 0 {
			ingress.Annotations["kubernetes.io/ingress.class"] = config.IngressClass
		}
		ingress.Annotations["kubernetes.io/tls-acme"] = "true"

		switch group {
		case tlsOptional:
			ingress.Annotations["ingress.kubernetes.io/ssl-redirect"] = "false"
			err := config.addHosts(&ingress, config.Plain, "")
			if err != nil {
				log.Fatalf("%s", err)
			}
			for tlsGroup, hostList := range config.TLSOptional {
				log.Printf("Adding TLS Optional group %s", tlsGroup)
				err = config.addHosts(&ingress, hostList, tlsGroup)
				if err != nil {
					log.Fatalf("%s", err)
				}
			}

		case tlsRequired:
			ingress.Annotations["ingress.kubernetes.io/ssl-redirect"] = "true"

			for tlsGroup, hostList := range config.TLSRequired {
				log.Printf("Adding TLS Optional group %s", tlsGroup)
				err = config.addHosts(&ingress, hostList, tlsGroup)
				if err != nil {
					log.Fatalf("%s", err)
				}
			}
		}

		ingressList.Items = append(ingressList.Items, ingress)
	}

	js, err := json.MarshalIndent(ingressList, "", "  ")
	if err != nil {
		log.Fatalf("Could not make JSON of ingress list: %s", err)
	}
	fmt.Printf("%s\n", js)

}

func (c *Config) addHosts(ingress *v1beta1.Ingress, hosts hostList, tlsName string) error {

	tls := v1beta1.IngressTLS{}

	if len(tlsName) > 0 {
		tls.SecretName = tlsName + "-tls"
	}

	for _, h := range hosts {
		rule := v1beta1.IngressRule{}
		rule.Host = h.Host
		if len(tlsName) > 0 {
			tls.Hosts = append(tls.Hosts, h.Host)
		}

		serviceName := h.ServiceName
		if len(serviceName) == 0 {
			serviceName = c.ServiceName
		}

		if len(serviceName) == 0 {
			return fmt.Errorf("service-name required")
		}

		servicePort := h.ServicePort
		if servicePort == 0 {
			servicePort = c.ServicePort
		}

		rule.HTTP = &v1beta1.HTTPIngressRuleValue{}
		rule.HTTP.Paths = []v1beta1.HTTPIngressPath{
			v1beta1.HTTPIngressPath{
				Backend: v1beta1.IngressBackend{
					ServiceName: serviceName,
					ServicePort: intstr.FromInt(servicePort),
				},
				Path: "/",
			},
		}
		ingress.Spec.Rules = append(ingress.Spec.Rules, rule)
	}
	if len(tlsName) > 0 {
		ingress.Spec.TLS = append(ingress.Spec.TLS, tls)
	}
	return nil
}
