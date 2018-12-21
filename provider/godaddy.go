package provider

import (
	"fmt"
	"net/http"

	"github.com/kubernetes-incubator/external-dns/endpoint"
	"github.com/kubernetes-incubator/external-dns/plan"
	ihttp "github.com/linki/instrumented_http"
	log "github.com/sirupsen/logrus"
)

const (
	GODADDY_ENV_OTE  = "ote"
	GODADDY_ENV_PROD = "prod"

	GODADDY_OTE_URL  = "https://api.ote-godaddy.com"
	GODADDY_PROD_URL = "https://api.godaddy.com"
)

//
type GoDaddyRecord struct {
	Name string `json:"name,omitempty"`
	Data string `json:"data,omitempty"`
	Type string `json:"type,omitempty"`
	TTL  int    `json:"ttl,omitempty"`

	Port     int    `json:"port,omitempty"`     // SRV Only
	Priority int    `json:"priority,omitempty"` // MX, SRV Only
	Protocol string `json:"protocol,omitempty"` // SRV Only
	Service  string `json:"service,omitempty"`  // SRV Only
	Weight   int    `json:"weight,omitempty"`   // SRV Only
}

func (g *GoDaddyRecord) ToEndpoint() *endpoint.Endpoint {
	return &endpoint.Endpoint{
		DNSName:    g.Name,
		Targets:    endpoint.NewTargets(g.Data),
		RecordType: g.Type,
		RecordTTL:  endpoint.TTL(g.TTL),
	}
}

func ToGoDaddyRecord(e *endpoint.Endpoint) (records []*GoDaddyRecord) {
	for _, target := range e.Targets {
		records = append(records, &GoDaddyRecord{
			Name: e.DNSName,
			Data: target,
			Type: e.RecordType,
			TTL:  int(e.RecordTTL),
		})
	}
	return records
}

type GoDaddyProvider struct {
	ApiEnv    string
	ApiKey    string
	ApiSecret string
	BaseUrl   string
	Client    *http.Client
	Filter    DomainFilter
}

func NewGoDaddyProvider(domainFilter DomainFilter, api_env string, api_key string, api_secret string, client *http.Client) (*GoDaddyProvider, error) {
	base_url := GODADDY_OTE_URL
	if api_env == GODADDY_ENV_PROD {
		base_url = GODADDY_PROD_URL
	}

	if client == nil {
		client = ihttp.NewClient(nil, &ihttp.Callbacks{
			PathProcessor:  ihttp.IdentityProcessor,
			QueryProcessor: ihttp.IdentityProcessor,
		})
	}

	return &GoDaddyProvider{
		ApiEnv:    api_env,
		ApiKey:    api_key,
		ApiSecret: api_secret,
		BaseUrl:   base_url,
		Client:    client,
		Filter:    domainFilter, // Filter.filters []string
	}, nil
}

func (p *GoDaddyProvider) Headers() map[string]string {
	return map[string]string{
		"Accept":        "application/json",
		"Authorization": fmt.Sprintf("sso-key %s:%s", p.ApiKey, p.ApiSecret),
		"User-Agent":    "k8s-external-dns",
	}
}

func (p *GoDaddyProvider) addHeaders(r *http.Request) {
	for k, v := range p.Headers() {
		log.Debugf("  Adding Request Headers[%s] = '%s'", k, v)
		r.Header.Set(k, v)
	}
}

func (p *GoDaddyProvider) makeRequest(r *http.Request) (*http.Response, error) {
	log.Debug("Making request...")
	p.addHeaders(r)
	return p.Client.Do(r)
}

func (p *GoDaddyProvider) Records() (endpoints []*endpoint.Endpoint, _ error) {
	log.Infof("Fetching DNS Records from GoDaddy (%s)", p.ApiEnv)
	for _, domain := range p.Filter.filters {
		for _, ep := range p.RecordsForDomain(domain) {
			endpoints = append(endpoints, ep)
		}
	}
	return endpoints, nil
}

func (p *GoDaddyProvider) RecordsForDomain(domain string) (eps []*endpoint.Endpoint) {
	log.Infof("  >> DNS records for domain '%s'", domain)
	full_path := p.url(domain, "records")
	log.Infof("     %s", full_path)
	// req, err := http.NewRequest(http.MethodGet, full_path, nil)
	// if err != nil {
	// 	return nil, err
	// }

	// resp, err := p.makeRequest(req)
	// if err != nil {
	// 	return nil, err
	// }
	// // for i, obj := range resp.Body.whatever {
	// //     endpoints = append(endpoints, obj)
	// // }
	return nil
}

func (p *GoDaddyProvider) ApplyChanges(changes *plan.Changes) error {
	log.Infof("Applying DNS Changes to GoDaddy (%s)", p.ApiEnv)
	return nil
}

func (p *GoDaddyProvider) url(domain string, path string) string {
	return fmt.Sprintf("%s/v1/domains/%s/%s", p.BaseUrl, domain, path)
}
