# Kube Ingress Template

For ingresses with lots of hostnames it's a hassle to keep the TLS and
Hosts sections in sync. If some hosts have TLS required, some optional
and others no TLS at all it's easy to get wrong. This tool might make
it a little easier.

From a JSON input with lists of hostnames it'll generate a JSON ingress manifest.

## Install

With Go installed, you can run

    go get -u github.com/abh/kube-ingress-template
    go install github.com/abh/kube-ingress-template

and the program should then be installed in $GOPATH/bin

## Sample data

The input file has to be in `ingress-hosts.json` in the current
directory.

```
{
  "name": "some-ingress",
  "namespace": "mynamespace",
  "service-name": "web-service",
  "service-port": 8080,
  "plain": [
    "no-tls.example.org",
    "plain.example.com"
  ],
  "tls-optional": {
    "example-net": [
      "tls-not-forced.example.net"
    ]
  },
  "tls-required": {
    "cert2": [
      "always-tls.example.com",
      "yup.example.org"
    ],
    "another-cert": [
      "www.example.org",
      "example.org"
    ]
  }
}
```
