package syncing

import (
	"fmt"
	"time"

	"github.com/RoundRobinHood/jouma-data-migration/types"
)

func SyncUp(wp_cnf, wc_cnf types.ApiConfig, TarsusProducts []types.TarsusProduct) {
	fmt.Println("Syncing categories...")
	SyncCategories(wc_cnf, TarsusProducts)
	time.Sleep(5 * time.Second)
	fmt.Println("Syncing products...")
	SyncProducts(wp_cnf, wc_cnf, TarsusProducts)
}
