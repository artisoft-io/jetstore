package registerkey

import (
	"context"

	"github.com/artisoft-io/jetstore/jets/datatable"
)

// Hook is a site's opportunity to add to the registration the stock path is about
// to make, or to take the event over entirely. A nil Hook is register_keys_v2
// exactly as it stands today.
//
// components is what utils.SplitFileKeyIntoComponents produced, with "size"
// already set from the S3 event, so a hook neither re-splits the key nor invents a
// size. It may add entries -- "schema_provider_json" is the one the first site to
// use this needed -- and this package builds the datatable.RegisterFileKeyAction
// and makes the call, so the Data-map key contract stays here rather than in a
// workspace. A site therefore never names "file_key", "year" or "size", and a
// rename of any of them here is a compile error rather than a silent no-op at the
// site.
//
// isSchemaEvent sets the RegisterFileKeyAction field of the same name. done ==
// true means the hook has handled the event itself and nothing further is
// registered, which is how a site reports a failure of its own without the stock
// registration also happening. A non-nil err is returned to the Lambda runtime
// unchanged, whatever done says.
//
// dtCtx.Dbpool is the pool, so a hook needs no second parameter for that.
type Hook interface {
	BeforeRegister(ctx context.Context, dtCtx *datatable.DataTableContext,
		fileKey string, components map[string]any,
		token string) (isSchemaEvent bool, done bool, err error)
}
