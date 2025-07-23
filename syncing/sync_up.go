package syncing

import (
	"github.com/RoundRobinHood/jouma-data-migration/types"
)

func SyncUp(wp_cnf, wc_cnf types.ApiConfig, TarsusProducts []types.TarsusProduct) {
	SyncProducts(wp_cnf, wc_cnf, TarsusProducts)
}
