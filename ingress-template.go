package main

//go:generate stringer -type=ingressGroup

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type hostList []string

type hostGroups map[string]hostList

type Config struct {
	Name        string     `json:"name"`
	Namespace   string     `json:"namespace"`
	ServiceName string     `json:"service-name"`
	ServicePort int        `json:"service-port"`
	Plain       hostList   `json:"plain"`
	TLSOptional hostGroups `json:"tls-optional"`
	TLSRequired hostGroups `json:"tls-required"`
}

type ingressGroup uint8

const (
	TLSOptional ingressGroup = iota
	TLSRequired
)

func main() {

	b, err := ioutil.ReadFile("ingress-hosts.json")
	if err != nil {
		log.Fatalf("could not read config: %s", err)
	}
	var config Config
	err = json.Unmarshal(b, &config)
	if err != nil {
		log.Fatalf("could not read config: %s", err)
	}
	// log.Printf("config: %+v", config)

	if len(config.ServiceName) == 0 {
		log.Fatalf("service-name required")
	}

	if config.ServicePort == 0 {
		config.ServicePort = 80
	}

	ingressList := v1beta1.IngressList{}
	ingressList.APIVersion = "v1"
	ingressList.Kind = "List"

	for _, group := range []ingressGroup{TLSOptional, TLSRequired} {

		for _, ingressClass := range []string{"haproxy"} {

			log.Printf("IngressClass %s, group %s", ingressClass, group)

			name := config.Name
			if group == TLSRequired {
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
			ingress.Annotations["kubernetes.io/ingress.class"] = "haproxy"
			ingress.Annotations["kubernetes.io/tls-acme"] = "true"

			switch group {
			case TLSOptional:
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

			case TLSRequired:
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
		rule.Host = h
		if len(tlsName) > 0 {
			tls.Hosts = append(tls.Hosts, h)
		}

		rule.HTTP = &v1beta1.HTTPIngressRuleValue{}
		rule.HTTP.Paths = []v1beta1.HTTPIngressPath{
			v1beta1.HTTPIngressPath{
				Backend: v1beta1.IngressBackend{
					ServiceName: c.ServiceName,
					ServicePort: intstr.FromInt(c.ServicePort),
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
