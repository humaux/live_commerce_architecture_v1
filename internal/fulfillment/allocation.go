package fulfillment

import (
	"context"
	"errors"
	"math"
	"sort"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// AllocationInput sets the warehouse priority of one logical delivery service.
type AllocationInput struct {
	MarketID               string   `json:"market_id"`
	Country                string   `json:"country"`
	Code                   string   `json:"code"`
	ExpectedVersion        int64    `json:"expected_version"`
	ExpectedServiceVersion int64    `json:"expected_service_version"`
	WarehouseIDs           []string `json:"warehouse_ids"`
}

type Allocation struct {
	MarketID       string   `json:"market_id"`
	Country        string   `json:"country"`
	Code           string   `json:"code"`
	Version        int64    `json:"version"`
	ServiceVersion int64    `json:"service_version"`
	WarehouseIDs   []string `json:"warehouse_ids"`
}

func SetAllocation(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string, in AllocationInput) (out Allocation, err error) {
	if !validAllocationInput(in) {
		return out, command.ErrInvalid
	}
	if err = authorize(ctx, tx, scope, token, managePermission); err != nil {
		return out, err
	}
	// Nil and empty are the same command and the same saved revision.
	in.WarehouseIDs = append(make([]string, 0, len(in.WarehouseIDs)), in.WarehouseIDs...)
	request := struct {
		PrincipalID string `json:"principal_id"`
		AllocationInput
	}{scope.PrincipalID, in}
	err = command.Run(ctx, tx, scope, "fulfillment.allocation.set", key, request, &out, func() error {
		var serviceVersion int64
		lockErr := tx.QueryRow(ctx, `SELECT current_version FROM fulfillment.service_heads
			WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND code=$5 FOR SHARE`,
			scope.TenantID, scope.StoreID, in.MarketID, in.Country, in.Code).Scan(&serviceVersion)
		if lockErr != nil {
			return mapError(lockErr)
		}
		if serviceVersion != in.ExpectedServiceVersion {
			return command.ErrConflict
		}
		lockKey := "fulfillment.allocation|" + scope.TenantID + "|" + scope.StoreID + "|" + in.MarketID + "|" + in.Country + "|" + in.Code
		if lockErr = advisoryLock(ctx, tx, lockKey); lockErr != nil {
			return lockErr
		}
		var currentVersion int64
		lockErr = tx.QueryRow(ctx, `SELECT current_version FROM fulfillment.allocation_heads
			WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND code=$5 FOR UPDATE`,
			scope.TenantID, scope.StoreID, in.MarketID, in.Country, in.Code).Scan(&currentVersion)
		exists := lockErr == nil
		if lockErr != nil && !errors.Is(lockErr, pgx.ErrNoRows) {
			return mapError(lockErr)
		}
		if exists != (in.ExpectedVersion > 0) || (exists && currentVersion != in.ExpectedVersion) {
			return command.ErrConflict
		}

		// Preference order is persisted; lock order is globally stable.
		sorted := append([]string(nil), in.WarehouseIDs...)
		sort.Strings(sorted)
		for _, id := range sorted {
			var active bool
			lockErr = tx.QueryRow(ctx, `SELECT active FROM inventory.lock_warehouse($1::uuid)`, id).Scan(&active)
			if lockErr != nil {
				return mapError(lockErr)
			}
			if !active {
				return command.ErrConflict
			}
		}

		version := int64(1)
		if exists {
			version = currentVersion + 1
		}
		out = Allocation{MarketID: in.MarketID, Country: in.Country, Code: in.Code,
			Version: version, ServiceVersion: serviceVersion, WarehouseIDs: in.WarehouseIDs}
		_, insertErr := tx.Exec(ctx, `INSERT INTO fulfillment.allocation_versions(
			tenant_id,store_id,market_id,country,code,version,service_version,warehouse_count,principal_id)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, scope.TenantID, scope.StoreID,
			in.MarketID, in.Country, in.Code, version, serviceVersion, len(in.WarehouseIDs), scope.PrincipalID)
		if insertErr != nil {
			return mapError(insertErr)
		}
		for i, id := range in.WarehouseIDs {
			_, insertErr = tx.Exec(ctx, `INSERT INTO fulfillment.allocation_warehouses(
				tenant_id,store_id,market_id,country,code,version,position,warehouse_id)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, scope.TenantID, scope.StoreID,
				in.MarketID, in.Country, in.Code, version, i+1, id)
			if insertErr != nil {
				return mapError(insertErr)
			}
		}
		if exists {
			tag, updateErr := tx.Exec(ctx, `UPDATE fulfillment.allocation_heads SET current_version=$6
				WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND code=$5 AND current_version=$7`,
				scope.TenantID, scope.StoreID, in.MarketID, in.Country, in.Code, version, currentVersion)
			if updateErr != nil {
				return mapError(updateErr)
			}
			if tag.RowsAffected() != 1 {
				return command.ErrConflict
			}
		} else {
			_, insertErr = tx.Exec(ctx, `INSERT INTO fulfillment.allocation_heads(
				tenant_id,store_id,market_id,country,code,current_version) VALUES($1,$2,$3,$4,$5,$6)`,
				scope.TenantID, scope.StoreID, in.MarketID, in.Country, in.Code, version)
			if insertErr != nil {
				return mapError(insertErr)
			}
		}
		return command.Audit(ctx, tx, scope, "fulfillment.allocation.set")
	})
	return out, mapError(err)
}

// ensureDefaultAllocation is the idempotent auto-allocation used by the settings-path enable/update
// (SetServiceWithDefaultAllocation). It runs in the caller's transaction and only acts when the
// service has no allocation head yet, so an explicit merchant warehouse priority is never overwritten.
// The service head was already locked FOR UPDATE by SetService, so the lock order stays globally
// consistent with SetAllocation: service head -> allocation advisory lock -> allocation head ->
// locked warehouse.
//
// It returns the allocation's first-position warehouse (the chosen default on create, the merchant's
// existing priority otherwise) so the response can show it; an empty allocation yields an empty string.
//
// It is deliberately not wrapped in command.Run: the allocation is a deterministic consequence of the
// already-replayed service write, and its existence check is the idempotency guard. A missing or
// inactive default warehouse is an explicit conflict (the service write rolls back with it), never a
// silent "enabled but no allocation".
func ensureDefaultAllocation(ctx context.Context, tx pgx.Tx, scope platform.Scope, service Service) (string, error) {
	lockKey := "fulfillment.allocation|" + scope.TenantID + "|" + scope.StoreID + "|" + service.MarketID + "|" + service.Country + "|" + service.Code
	if err := advisoryLock(ctx, tx, lockKey); err != nil {
		return "", err
	}
	var currentVersion int64
	lockErr := tx.QueryRow(ctx, `SELECT current_version FROM fulfillment.allocation_heads
		WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND code=$5 FOR UPDATE`,
		scope.TenantID, scope.StoreID, service.MarketID, service.Country, service.Code).Scan(&currentVersion)
	if lockErr == nil {
		// An allocation already exists: keep the merchant's own priority and just report its
		// first warehouse (empty when the merchant chose an empty allocation).
		var existing string
		if err := tx.QueryRow(ctx, `SELECT warehouse_id::text FROM fulfillment.allocation_warehouses
			WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND code=$5 AND version=$6 AND position=1`,
			scope.TenantID, scope.StoreID, service.MarketID, service.Country, service.Code, currentVersion).Scan(&existing); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return "", nil
			}
			return "", mapError(err)
		}
		return existing, nil
	}
	if !errors.Is(lockErr, pgx.ErrNoRows) {
		return "", mapError(lockErr)
	}

	// Default warehouse: the store's sole active warehouse, else the first active by creation order.
	var warehouseID string
	if err := tx.QueryRow(ctx, `SELECT w.id::text FROM inventory.warehouses w
		WHERE w.tenant_id=$1 AND w.store_id=$2 AND w.active
		ORDER BY w.created_at, w.id LIMIT 1`, scope.TenantID, scope.StoreID).Scan(&warehouseID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", command.ErrConflict
		}
		return "", mapError(err)
	}
	// Lock and re-check the chosen warehouse, mirroring SetAllocation's active check.
	var active bool
	if err := tx.QueryRow(ctx, `SELECT active FROM inventory.lock_warehouse($1::uuid)`, warehouseID).Scan(&active); err != nil {
		return "", mapError(err)
	}
	if !active {
		return "", command.ErrConflict
	}

	if _, err := tx.Exec(ctx, `INSERT INTO fulfillment.allocation_versions(
		tenant_id,store_id,market_id,country,code,version,service_version,warehouse_count,principal_id)
		VALUES($1,$2,$3,$4,$5,1,$6,1,$7)`, scope.TenantID, scope.StoreID,
		service.MarketID, service.Country, service.Code, service.Version, scope.PrincipalID); err != nil {
		return "", mapError(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO fulfillment.allocation_warehouses(
		tenant_id,store_id,market_id,country,code,version,position,warehouse_id)
		VALUES($1,$2,$3,$4,$5,1,1,$6)`, scope.TenantID, scope.StoreID,
		service.MarketID, service.Country, service.Code, warehouseID); err != nil {
		return "", mapError(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO fulfillment.allocation_heads(
		tenant_id,store_id,market_id,country,code,current_version) VALUES($1,$2,$3,$4,$5,1)`,
		scope.TenantID, scope.StoreID, service.MarketID, service.Country, service.Code); err != nil {
		return "", mapError(err)
	}
	if err := command.Audit(ctx, tx, scope, "fulfillment.allocation.ensure"); err != nil {
		return "", err
	}
	return warehouseID, nil
}

func GetAllocation(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, marketID, country, code string) (out Allocation, err error) {
	if !command.ValidID(marketID) || !countryPattern.MatchString(country) || !codePattern.MatchString(code) {
		return out, command.ErrInvalid
	}
	if err = authorize(ctx, tx, scope, token, readPermission); err != nil {
		return out, err
	}
	err = tx.QueryRow(ctx, `SELECT v.market_id::text,v.country,v.code,v.version,v.service_version
		FROM fulfillment.allocation_heads h JOIN fulfillment.allocation_versions v
		ON (v.tenant_id,v.store_id,v.market_id,v.country,v.code,v.version)=
		(h.tenant_id,h.store_id,h.market_id,h.country,h.code,h.current_version)
		WHERE h.tenant_id=$1 AND h.store_id=$2 AND h.market_id=$3 AND h.country=$4 AND h.code=$5`,
		scope.TenantID, scope.StoreID, marketID, country, code).Scan(
		&out.MarketID, &out.Country, &out.Code, &out.Version, &out.ServiceVersion)
	if err != nil {
		return Allocation{}, mapError(err)
	}
	rows, err := tx.Query(ctx, `SELECT warehouse_id::text FROM fulfillment.allocation_warehouses
		WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND code=$5 AND version=$6
		ORDER BY position`, scope.TenantID, scope.StoreID, marketID, country, code, out.Version)
	if err != nil {
		return Allocation{}, mapError(err)
	}
	defer rows.Close()
	out.WarehouseIDs = make([]string, 0)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return Allocation{}, mapError(err)
		}
		out.WarehouseIDs = append(out.WarehouseIDs, id)
	}
	if err = rows.Err(); err != nil {
		return Allocation{}, mapError(err)
	}
	return out, nil
}

func validAllocationInput(in AllocationInput) bool {
	if !command.ValidID(in.MarketID) || !countryPattern.MatchString(in.Country) || !codePattern.MatchString(in.Code) ||
		in.ExpectedVersion < 0 || in.ExpectedVersion == math.MaxInt64 || in.ExpectedServiceVersion < 1 ||
		len(in.WarehouseIDs) > 16 {
		return false
	}
	seen := make(map[string]bool, len(in.WarehouseIDs))
	for _, id := range in.WarehouseIDs {
		if !command.ValidID(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
