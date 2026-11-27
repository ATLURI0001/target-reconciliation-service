package dispatch

import "fmt"

// TargetWeightContract validates dispatch target routing weights.
type TargetWeightContract struct {
	MinWeight int
	MaxWeight int
}

func NewTargetWeightContract(minW, maxW int) *TargetWeightContract {
	return &TargetWeightContract{MinWeight: minW, MaxWeight: maxW}
}

func (c *TargetWeightContract) Validate(weight int) error {
	if weight < c.MinWeight || weight > c.MaxWeight {
		return fmt.Errorf("weight %d out of range [%d, %d]", weight, c.MinWeight, c.MaxWeight)
	}
	return nil
}
