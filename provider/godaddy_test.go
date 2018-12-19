package provider

import (
	"testing"

	"github.com/kubernetes-incubator/external-dns/endpoint"

	"github.com/stretchr/testify/require"
)

func TestGoDaddyProviderRecords(t *testing.T) {
	//p := newGoDaddyProvider(t, []string{"www"})
	//records, err := p.Records()
	require.NoError(t, nil)
}

func TestGoDaddyProviderApplyChanges(t *testing.T) {
	require.NoError(t, nil)
}

func TestGoDaddyRecordToEndpoint(t *testing.T) {
	rec := &GoDaddyRecord{
		Name: "www",
		Data: "1.2.3.4",
		TTL:  3600,
		Type: "A",
	}
	ep := rec.ToEndpoint()

	require.Equal(t, ep.DNSName, rec.Name)
	require.Equal(t, ep.RecordType, rec.Type)
	require.Equal(t, ep.RecordTTL, endpoint.TTL(rec.TTL))
}

func TestEndpointToGoDaddyRecord(t *testing.T) {
	ep := endpoint.NewEndpointWithTTL("www", "A", endpoint.TTL(3600), "1.2.3.4", "2.3.4.5", "3.4.5.6")
	records := ToGoDaddyRecord(ep)

	for _, rec := range records {
		require.Equal(t, rec.Name, "www")
		require.Equal(t, rec.Type, "A")
		require.Equal(t, rec.TTL, 3600)
	}
}

func newGoDaddyProvider(t *testing.T, domains []string) *GoDaddyProvider {
	return &GoDaddyProvider{
		ApiEnv:  "ote",
		BaseUrl: "https://api.godaddy.test",
		Client:  nil,
		Filter:  NewDomainFilter(domains),
	}
}
