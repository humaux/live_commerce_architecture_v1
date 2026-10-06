package catalog

// Purpose: product-media-v2 commands that are not plain upload/delete/reorder (contracts/catalog-inventory-v1.md "Amendment —
// product-media-v2"): the effective image axis, option-value image links, moving an image between the main and detail roles, and
// the image-axis setter. Pure helpers (effectiveAxis, axisHasValue) are unit-tested without a database.
// Depends on: internal/command (idempotent Run, Audit), internal/platform.Scope; tables catalog.product_images, catalog.product_option_images
//   and catalog.products.image_axis (migration 0149; role commerce_runtime, FORCE RLS scope_access); editableProduct/lockProduct (catalog.go).
// Used by: internal/httpapi/images.go (POST images/{id}/move, option-images, image-axis); images.go (UploadImage, readImages).
// Invariants: every mutation holds the product row lock first (same order as images.go); caps are Go (roleCap) and SQL (CHECK + UNIQUE).

import (
	"context"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// MoveInput is POST .../images/{id}/move.
type MoveInput struct {
	Role string `json:"role"`
}

// LinkInput is POST .../option-images: point a sku image at a value of the effective image axis.
type LinkInput struct {
	OptionValue string `json:"option_value"`
	ImageID     string `json:"image_id"`
}

// AxisInput is POST .../image-axis; a nil Axis returns to the default (the first axis).
type AxisInput struct {
	Axis *string `json:"axis"`
}

// effectiveAxis is the axis that carries option-value images: the stored image_axis when it names a current axis, else the first
// axis, else none (a product without options has no sku images). Mirrors catalog.sku_option_image / buyer_v2_product in SQL.
func effectiveAxis(axes []OptionAxis, stored *string) (string, bool) {
	if stored != nil {
		for _, a := range axes {
			if a.Name == *stored {
				return a.Name, true
			}
		}
	}
	if len(axes) > 0 {
		return axes[0].Name, true
	}
	return "", false
}

func axisHasValue(axes []OptionAxis, axis, value string) bool {
	for _, a := range axes {
		if a.Name != axis {
			continue
		}
		for _, v := range a.Values {
			if v == value {
				return true
			}
		}
	}
	return false
}

// productImageAxis reads the option axes and the stored image axis of a product in the caller's scope.
func productImageAxis(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID string) ([]OptionAxis, *string, error) {
	var axes []OptionAxis
	var stored *string
	// catalog.products (commerce_runtime): options + image_axis (migration 0149).
	err := tx.QueryRow(ctx, `SELECT options,image_axis FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND id=$3`,
		scope.TenantID, scope.StoreID, productID).Scan(&axes, &stored)
	return axes, stored, mapError(err)
}

// checkOptionValue resolves the effective image axis of a product whose row the caller has locked and requires value to be on it
// (ErrInvalid otherwise, including a product without options). It returns the axis name to key the link by.
func checkOptionValue(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID string, axes []OptionAxis, value string) (string, error) {
	var stored *string
	if err := tx.QueryRow(ctx, `SELECT image_axis FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, scope.TenantID, scope.StoreID, productID).Scan(&stored); err != nil {
		return "", mapError(err)
	}
	axis, ok := effectiveAxis(axes, stored)
	if !ok || !axisHasValue(axes, axis, value) {
		return "", command.ErrInvalid
	}
	return axis, nil
}

// MoveImage moves a main or detail image to the END of the other role (bytes and size children stay), renumbering the source role.
// ErrInvalid: a sku image, the same role, or a detail target taller than 6x its width; ErrConflict: target role full or archived product.
func MoveImage(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, productID, imageID string, in MoveInput) (out ImageList, err error) {
	if !command.ValidID(productID) || !command.ValidID(imageID) || (in.Role != RoleMain && in.Role != RoleDetail) {
		return out, command.ErrInvalid
	}
	request := struct {
		ProductID string `json:"product_id"`
		ImageID   string `json:"image_id"`
		Role      string `json:"role"`
	}{productID, imageID, in.Role}
	err = command.Run(ctx, tx, scope, "catalog.image.move", key, request, &out, func() error {
		if _, err := editableProduct(ctx, tx, scope, productID); err != nil {
			return err
		}
		var role string
		var position int
		var width, height *int
		// catalog.product_images (commerce_runtime): metadata only.
		if err := tx.QueryRow(ctx, `SELECT role,position,width,height FROM catalog.product_images WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND id=$4`,
			scope.TenantID, scope.StoreID, productID, imageID).Scan(&role, &position, &width, &height); err != nil {
			return err
		}
		if role == RoleSKU || role == in.Role || (in.Role == RoleDetail && tooTall(width, height)) {
			return command.ErrInvalid
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM catalog.product_images WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND role=$4`,
			scope.TenantID, scope.StoreID, productID, in.Role).Scan(&count); err != nil {
			return err
		}
		if count >= roleCap(in.Role) {
			return command.ErrConflict
		}
		// Column grant UPDATE(role,position,version): the deferred UNIQUE (role,position) lets the move and the renumber share one transaction.
		if _, err := tx.Exec(ctx, `UPDATE catalog.product_images SET role=$5,position=$6,version=version+1 WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND id=$4`,
			scope.TenantID, scope.StoreID, productID, imageID, in.Role, count); err != nil {
			return err
		}
		if err := closeGap(ctx, tx, scope, productID, role, position); err != nil {
			return err
		}
		if err := command.Audit(ctx, tx, scope, "catalog.product.image_moved"); err != nil {
			return err
		}
		out, err = readImages(ctx, tx, scope, productID)
		return err
	})
	return out, mapError(err)
}

// LinkOptionImage points a sku-role image of the product at a value of the effective image axis, replacing the image's previous link.
// ErrConflict when the value already has ANOTHER image; ErrInvalid when the value is not on the axis or the image is not role sku;
// ErrNotFound for a foreign product or image (scoped lookup).
func LinkOptionImage(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, productID string, in LinkInput) (out ImageList, err error) {
	if !command.ValidID(productID) || !command.ValidID(in.ImageID) || !plainText(in.OptionValue, maxAxisValue) {
		return out, command.ErrInvalid
	}
	request := struct {
		ProductID   string `json:"product_id"`
		ImageID     string `json:"image_id"`
		OptionValue string `json:"option_value"`
	}{productID, in.ImageID, in.OptionValue}
	err = command.Run(ctx, tx, scope, "catalog.image.link", key, request, &out, func() error {
		axes, err := editableProduct(ctx, tx, scope, productID)
		if err != nil {
			return err
		}
		var role string
		if err := tx.QueryRow(ctx, `SELECT role FROM catalog.product_images WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND id=$4`,
			scope.TenantID, scope.StoreID, productID, in.ImageID).Scan(&role); err != nil {
			return err
		}
		if role != RoleSKU {
			return command.ErrInvalid
		}
		axis, err := checkOptionValue(ctx, tx, scope, productID, axes, in.OptionValue)
		if err != nil {
			return err
		}
		var holder *string
		err = tx.QueryRow(ctx, `SELECT image_id::text FROM catalog.product_option_images WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND option_name=$4 AND option_value=$5`,
			scope.TenantID, scope.StoreID, productID, axis, in.OptionValue).Scan(&holder)
		switch {
		case err == nil && *holder == in.ImageID:
			// already linked to this value: nothing to change
		case err == nil:
			return command.ErrConflict
		case mapError(err) == command.ErrNotFound:
			// catalog.product_option_images (commerce_runtime): re-point = drop the image's old link, add the new one.
			if _, err := tx.Exec(ctx, `DELETE FROM catalog.product_option_images WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND image_id=$4`,
				scope.TenantID, scope.StoreID, productID, in.ImageID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO catalog.product_option_images(tenant_id,store_id,product_id,option_name,option_value,image_id) VALUES($1,$2,$3,$4,$5,$6)`,
				scope.TenantID, scope.StoreID, productID, axis, in.OptionValue, in.ImageID); err != nil {
				return err
			}
		default:
			return err
		}
		if err := command.Audit(ctx, tx, scope, "catalog.product.image_linked"); err != nil {
			return err
		}
		out, err = readImages(ctx, tx, scope, productID)
		return err
	})
	return out, mapError(err)
}

// SetImageAxis selects which option axis carries option-value images (nil = the first axis). The name must be a current axis
// (ErrInvalid otherwise). Links keyed by another axis stay stored but dormant and return if that axis is selected again.
func SetImageAxis(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, productID string, in AxisInput) (out ImageList, err error) {
	if !command.ValidID(productID) || (in.Axis != nil && !plainText(*in.Axis, maxAxisName)) {
		return out, command.ErrInvalid
	}
	request := struct {
		ProductID string  `json:"product_id"`
		Axis      *string `json:"axis"`
	}{productID, in.Axis}
	err = command.Run(ctx, tx, scope, "catalog.image.axis", key, request, &out, func() error {
		axes, err := editableProduct(ctx, tx, scope, productID)
		if err != nil {
			return err
		}
		if in.Axis != nil {
			found := false
			for _, a := range axes {
				found = found || a.Name == *in.Axis
			}
			if !found {
				return command.ErrInvalid
			}
		}
		// catalog.products (commerce_runtime): image_axis only; the product version is not bumped (it guards the document, not media).
		if _, err := tx.Exec(ctx, `UPDATE catalog.products SET image_axis=$4 WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, scope.TenantID, scope.StoreID, productID, in.Axis); err != nil {
			return err
		}
		if err := command.Audit(ctx, tx, scope, "catalog.product.image_axis_set"); err != nil {
			return err
		}
		out, err = readImages(ctx, tx, scope, productID)
		return err
	})
	return out, mapError(err)
}
