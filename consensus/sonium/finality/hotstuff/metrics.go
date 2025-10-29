package hotstuff
import "time"

// trivial EMA-based RTT estimator — wire with your p2p ping stats
//type Metrics struct{ rtt time.Duration }

type Metrics struct {
    FinalityLatency prometheus.Histogram
    ViewChange      prometheus.Counter
    VotesTotal      prometheus.Counter
    DroppedMsgs     prometheus.Counter
}

func NewMetrics() *Metrics { return &Metrics{rtt: 10 * time.Millisecond} }
func (m *Metrics) UpdateRTT(sample time.Duration) {
	m.rtt = (m.rtt*3 + sample) / 4
}
func (m *Metrics) P95RTT() time.Duration { return m.rtt } // stub


