package eval

// Estimate is a rough price for an LLM eval run, shown before asking the owner
// to confirm. It is deliberately a little pessimistic: the real cost is
// printed after the run.
type Estimate struct {
	Scorings int
	CostUSD  float64
}

// Pricing is USD per million tokens for the model being evaluated.
type Pricing struct{ Input, Output, CacheWrite, CacheRead float64 }

// EstimateCost guesses the cost of scoring every case `repeats` times.
// Per scoring we assume 3 model calls: the cached system prompt is read each
// time, the ad text is sent in two of them (about 4 characters per token), and
// about 700 tokens are written back. The system prompt is written to the cache
// once for the whole run.
func EstimateCost(cases []Case, repeats, systemChars int, p Pricing) Estimate {
	const perM = 1e6
	if repeats < 1 {
		repeats = 1
	}
	sysTokens := float64(systemChars) / 4
	cost := sysTokens * p.CacheWrite / perM // one cache write for the run
	n := 0
	for _, c := range cases {
		adTokens := float64(len(c.Ad)) / 4
		for r := 0; r < repeats; r++ {
			n++
			in := adTokens*2 + 900
			cost += in*p.Input/perM + 700*p.Output/perM + sysTokens*3*p.CacheRead/perM
		}
	}
	return Estimate{Scorings: n, CostUSD: cost}
}
