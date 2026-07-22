package vm

import "github.com/gobuffalo/plush/v5"

func (vm *VM) budget() *plush.Budget {
	if vm.ctx == nil {
		return nil
	}
	if provider, ok := vm.ctx.(interface{ Budget() *plush.Budget }); ok {
		return provider.Budget()
	}
	return nil
}

func (vm *VM) spendLoop() error {
	return vm.budget().SpendLoop()
}

func (vm *VM) spendCondition() error {
	return vm.budget().SpendCondition()
}

func (vm *VM) spendAssignment() error {
	return vm.budget().SpendAssignment()
}

func (vm *VM) spendFunctionCall(name string) error {
	if name == "" {
		name = anonymousCallName
	}
	return vm.budget().SpendFunctionCall(name)
}

func (vm *VM) spendTraversal(segments int) error {
	return vm.budget().SpendObjectTraversal(segments)
}
