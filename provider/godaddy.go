package provider

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

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

func (g *GoDaddyRecord) Pretty() string {
	return fmt.Sprintf("%s %s %s", g.Type, g.Name, g.Data)
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
		ApiKey:    strings.TrimSpace(api_key),
		ApiSecret: strings.TrimSpace(api_secret),
		BaseUrl:   base_url,
		Client:    client,
		Filter:    domainFilter, // Filter.filters []string
	}, nil
}

// Records returns a list of endpoints in GoDaddy matching the DomainFilter
func (p *GoDaddyProvider) Records() (endpoints []*endpoint.Endpoint, _ error) {
	log.Infof("Fetching DNS Records from GoDaddy (%s)", p.ApiEnv)
	for _, domain := range p.Filter.filters {
		endpoints = append(endpoints, p.RecordsForDomain(domain)...)
	}
	return endpoints, nil
}

// Headers returns a map of HTTP headers to use for the GoDaddy API Request
func (p *GoDaddyProvider) Headers() map[string]string {
	return map[string]string{
		"Accept":        "application/json",
		"Authorization": fmt.Sprintf("sso-key %s:%s", p.ApiKey, p.ApiSecret),
		"User-Agent":    "k8s-external-dns",
	}
}

// addHeaders adds headers to the *http.Request.
func (p *GoDaddyProvider) addHeaders(r *http.Request) {
	for k, v := range p.Headers() {
		r.Header.Set(k, v)
	}
}

// makeRequest prepares headers and makes the HTTP request to GoDaddy
// (in lieu of an actual API client)
func (p *GoDaddyProvider) makeRequest(r *http.Request) (*http.Response, error) {
	log.Debug("Making request...")
	p.addHeaders(r)
	return p.Client.Do(r)
}

func (p *GoDaddyProvider) RecordsForDomain(domain string) (eps []*endpoint.Endpoint) {
	log.Debugf("  >> DNS records for domain '%s'", domain)
	full_path := p.url(domain, "records")
	log.Debugf("     %s", full_path)

	req, err := http.NewRequest(http.MethodGet, full_path, nil)
	if err != nil {
		log.Fatalf("Error creating request: %s", err.Error())
	}

	resp, err := p.makeRequest(req)
	if err != nil {
		log.Fatalf("Error making request: %s", err.Error())
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error with response body: %s", err.Error())
	}

	records := []GoDaddyRecord{}
	err = json.Unmarshal(body, &records)
	if err != nil {
		log.Fatalf("Error with response: %s", err.Error())
	}

	// This should always return ALL records, not just those
	// managed by external-dns
	for _, rec := range records {
		ep := rec.ToEndpoint()
		log.Debugf("Got record: %s", ep.String())
		eps = append(eps, ep)
	}
	return eps
}

func (p *GoDaddyProvider) ApplyChanges(changes *plan.Changes) error {
	log.Infof("Applying Changes...    [Create (%d), Update (%d), Delete (%d)]", len(changes.Create), len(changes.UpdateNew), len(changes.Delete))

	p.prepareChanges(changes.Create, changes.UpdateNew, changes.Delete)

	log.Infof("Done Applying Changes")
	return nil
}

func (p *GoDaddyProvider) prepareChanges(create []*endpoint.Endpoint, update []*endpoint.Endpoint, delete []*endpoint.Endpoint) {
	current, err := p.Records()
	if err != nil {
		log.Fatal(err)
	}

	create = p.filterChanges(create)
	update = p.filterChanges(update)
	delete = p.filterChanges(delete)

	for _, ep := range current {
		log.Debugf("CURRENT: %+v", ep)
	}

	for _, ep := range create {
		log.Debugf("CREATE: %+v", ep)
	}

	for _, ep := range update {
		log.Debugf("UPDATE: %+v", ep)
	}

	for _, ep := range delete {
		log.Debugf("DELETE: %+v", ep)
	}
}

func (p *GoDaddyProvider) url(domain string, path string) string {
	return fmt.Sprintf("%s/v1/domains/%s/%s", p.BaseUrl, domain, path)
}

func (p *GoDaddyProvider) filterChanges(changes []*endpoint.Endpoint) []*endpoint.Endpoint {
	ret := make([]*endpoint.Endpoint, 0)
	for _, ch := range changes {
		if !p.Filter.Match(ch.DNSName) {
			log.Debugf("omitting change %s", ch.String())
			continue
		}
		ret = append(ret, ch)
	}
	return ret
}
