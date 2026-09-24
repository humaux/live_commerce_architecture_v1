package inventory

import (
	"sort"

	"livecommerce/internal/command"
)

type Demand struct {
	SKUID    string `json:"sku_id"`
	Quantity int64  `json:"quantity"`
}

// PlanAllocation is pure arithmetic over balances already locked and validated
// by the caller's checkout transaction. Priority chooses stock; output uses
// the global warehouse/SKU lock order.
func PlanAllocation(warehouseIDs []string, demands []Demand, balances []Balance) ([]Line, error) {
	if len(warehouseIDs) < 1 || len(warehouseIDs) > 16 || len(demands) < 1 || len(demands) > 50 || len(balances) > 800 {
		return nil, command.ErrInvalid
	}
	warehouses := make(map[string]bool, len(warehouseIDs))
	for _, id := range warehouseIDs {
		if !command.ValidID(id) || warehouses[id] {
			return nil, command.ErrInvalid
		}
		warehouses[id] = true
	}
	demandBySKU := make(map[string]int64, len(demands))
	for _, demand := range demands {
		if !command.ValidID(demand.SKUID) || demand.Quantity < 1 || demand.Quantity > command.MaxQuantity {
			return nil, command.ErrInvalid
		}
		if _, exists := demandBySKU[demand.SKUID]; exists {
			return nil, command.ErrInvalid
		}
		demandBySKU[demand.SKUID] = demand.Quantity
	}
	available := make(map[string]int64, len(balances))
	for _, balance := range balances {
		if !warehouses[balance.WarehouseID] || !command.ValidID(balance.SKUID) {
			return nil, command.ErrInvalid
		}
		if _, exists := demandBySKU[balance.SKUID]; !exists {
			return nil, command.ErrInvalid
		}
		if balance.OnHand < 0 || balance.OnHand > command.MaxQuantity ||
			balance.Reserved < 0 || balance.Reserved > command.MaxQuantity ||
			balance.Allocated < 0 || balance.Allocated > command.MaxQuantity ||
			balance.Unavailable < 0 || balance.Unavailable > command.MaxQuantity || balance.Version < 0 {
			return nil, command.ErrInvalid
		}
		free := balance.OnHand
		for _, used := range [...]int64{balance.Reserved, balance.Allocated, balance.Unavailable} {
			if used > free {
				return nil, command.ErrInvalid
			}
			free -= used
		}
		pair := balance.WarehouseID + balance.SKUID
		if _, exists := available[pair]; exists {
			return nil, command.ErrInvalid
		}
		available[pair] = free // Available is a redundant, untrusted projection.
	}
	skus := make([]string, 0, len(demandBySKU))
	for id := range demandBySKU {
		skus = append(skus, id)
	}
	sort.Strings(skus)
	plan := make([]Line, 0, len(balances))
	for _, sku := range skus {
		remaining := demandBySKU[sku]
		for _, warehouse := range warehouseIDs {
			quantity := min(remaining, available[warehouse+sku])
			if quantity > 0 {
				plan = append(plan, Line{WarehouseID: warehouse, SKUID: sku, Quantity: quantity})
				remaining -= quantity
			}
			if remaining == 0 {
				break
			}
		}
		if remaining > 0 {
			return nil, command.ErrInsufficient
		}
	}
	sort.Slice(plan, func(i, j int) bool {
		if plan[i].WarehouseID == plan[j].WarehouseID {
			return plan[i].SKUID < plan[j].SKUID
		}
		return plan[i].WarehouseID < plan[j].WarehouseID
	})
	return plan, nil
}
