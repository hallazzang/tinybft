package types

type ValidatorSet struct {
	Validators       []*Validator
	Proposer         *Validator
	totalVotingPower int64
	i                int
}

func NewValidatorSet(vals ...*Validator) *ValidatorSet {
	if len(vals) == 0 {
		panic("vals must not be empty")
	}
	return &ValidatorSet{
		Validators: vals,
	}
}

func (vs *ValidatorSet) Size() int {
	return len(vs.Validators)
}

func (vs *ValidatorSet) TotalVotingPower() int64 {
	if vs.totalVotingPower == 0 {
		vs.updateTotalVotingPower()
	}
	return vs.totalVotingPower
}

func (vs *ValidatorSet) updateTotalVotingPower() {
	sum := int64(0)
	for _, val := range vs.Validators {
		sum += val.VotingPower // overflow not considered
		// MaxTotalVotingPower not considered
	}
	vs.totalVotingPower = sum
}

func (vs *ValidatorSet) HasAddress(addr Address) bool {
	for _, val := range vs.Validators {
		if val.Address.Equal(addr) {
			return true
		}
	}
	return false
}

func (vs *ValidatorSet) GetByIndex(i int32) (addr Address, val *Validator) {
	if i < 0 || int(i) >= len(vs.Validators) {
		return Address{}, nil
	}
	val = vs.Validators[i]
	return val.Address, val.Copy()
}

func (vs *ValidatorSet) GetByAddress(addr Address) (idx int32, val *Validator) {
	for i, val := range vs.Validators {
		if val.Address.Equal(addr) {
			return int32(i), val.Copy()
		}
	}
	return -1, nil
}

func (vs *ValidatorSet) GetProposer() *Validator {
	if len(vs.Validators) == 0 {
		return nil
	}
	if vs.Proposer == nil {
		vs.Proposer = vs.findProposer()
	}
	return vs.Proposer.Copy()
}

func (vs *ValidatorSet) findProposer() *Validator {
	return vs.Validators[vs.i]
}

func (vs *ValidatorSet) Copy() *ValidatorSet {
	var vals []*Validator
	for _, val := range vs.Validators {
		vals = append(vals, val.Copy())
	}
	return &ValidatorSet{
		Validators: vals,
		Proposer:   vs.Proposer,
	}
}

// TODO: is it correct mock implementation?
func (vs *ValidatorSet) IncrementProposerPriority(times int32) {
	vs.i = (vs.i + int(times)) % len(vs.Validators)
}
