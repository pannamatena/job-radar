package sources

import (
	"fmt"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/httpclient"
)

// ErrNoPublicAPI means the company uses an ATS with no public feed (ats: none);
// it is covered by job-alert emails (phase 5), not fetched directly.
var ErrNoPublicAPI = fmt.Errorf("no public API")

// For returns the Source for a company based on its configured ATS. It returns
// ErrNoPublicAPI for ats: none, and a descriptive error for anything unknown.
func For(client *httpclient.Client, company config.Company) (Source, error) {
	switch company.ATS {
	case config.ATSGreenhouse:
		return NewGreenhouse(client, company), nil
	case config.ATSLever:
		return NewLever(client, company), nil
	case config.ATSAshby:
		return NewAshby(client, company), nil
	case config.ATSNone:
		return nil, ErrNoPublicAPI
	default:
		return nil, fmt.Errorf("unknown ats %q", company.ATS)
	}
}
