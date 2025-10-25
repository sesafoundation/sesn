package params

type FinalityConfig struct {
	Type          string `json:"type"`          // "hotstuff"
	ActivateAt    uint64 `json:"activateAt"`    // block where gadget turns on
	CommitteeSize uint64 `json:"committeeSize"` // 0 = all active
	TimeoutMS     uint64 `json:"timeoutMs"`     // 100–200ms
	BLS           bool   `json:"bls"`           // true
}