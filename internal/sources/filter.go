package sources

import (
	"strings"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/posting"
)

// filterMeta is the small bundle of structured facts a source knows about a job
// that the company filters act on. Each source fills it in from its own API
// shape, then calls passesFilters, so the filter rules live in one place rather
// than being re-implemented per source.
type filterMeta struct {
	Departments []string           // department/team names the job belongs to
	OfficeIDs   []int64            // structured office ids (Greenhouse only)
	Location    string             // free-text location
	Remote      posting.RemoteKind // derived work model
}

// passesFilters applies a company's configured filters. Two independent gates
// must both pass:
//
//   - department gate: if any departments are configured, the job must match one.
//   - location gate: if any of office_ids / location_names / include_remote_open_to
//     are configured, the job must satisfy at least one of them (OR).
//
// A company with no filters keeps everything. Filters only ever narrow; they
// never consider a posting's age (ADR-0025).
func passesFilters(m filterMeta, f config.CompanyFilters) bool {
	if len(f.Departments) > 0 && !departmentMatches(m.Departments, f.Departments) {
		return false
	}
	if locationGateConfigured(f) && !locationGateMatches(m, f) {
		return false
	}
	return true
}

func departmentMatches(jobDepts, want []string) bool {
	for _, w := range want {
		wl := strings.ToLower(strings.TrimSpace(w))
		if wl == "" {
			continue
		}
		for _, d := range jobDepts {
			if strings.Contains(strings.ToLower(d), wl) {
				return true
			}
		}
	}
	return false
}

func locationGateConfigured(f config.CompanyFilters) bool {
	return len(f.OfficeIDs) > 0 || len(f.LocationNames) > 0 || len(f.IncludeRemoteOpenTo) > 0
}

func locationGateMatches(m filterMeta, f config.CompanyFilters) bool {
	// Structured office-id match (Greenhouse).
	if len(f.OfficeIDs) > 0 && len(m.OfficeIDs) > 0 {
		want := make(map[int64]bool, len(f.OfficeIDs))
		for _, id := range f.OfficeIDs {
			want[id] = true
		}
		for _, id := range m.OfficeIDs {
			if want[id] {
				return true
			}
		}
	}

	locLower := strings.ToLower(m.Location)

	// Location-name substring match.
	for _, name := range f.LocationNames {
		if nl := strings.ToLower(strings.TrimSpace(name)); nl != "" && strings.Contains(locLower, nl) {
			return true
		}
	}

	// Remote role open to one of the configured regions.
	if len(f.IncludeRemoteOpenTo) > 0 && isRemoteWork(m.Remote) {
		for _, region := range f.IncludeRemoteOpenTo {
			if rl := strings.ToLower(strings.TrimSpace(region)); rl != "" && strings.Contains(locLower, rl) {
				return true
			}
		}
	}
	return false
}

func isRemoteWork(r posting.RemoteKind) bool {
	return r == posting.RemoteYes || r == posting.RemoteHybrid
}
