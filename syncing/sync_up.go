package syncing

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/RoundRobinHood/jouma-data-migration/types"
	"github.com/RoundRobinHood/jouma-data-migration/wc"
)

func SyncUp(cnf types.ApiConfig, TarsusProducts []types.TarsusProduct) {
	// Used to quickly check SKUs against tarsus products
	lookup := map[string]types.TarsusProduct{}

	// Delete a key when you find it (leftovers have to be created on WC)
	createCache := map[string]struct{}{}

	// List of IDs to be deleted
	deleteList := make([]int, 0)

	for _, product := range TarsusProducts {
		lookup[product.ProductNumber] = product
		createCache[product.ProductNumber] = struct{}{}
	}

	products, errors := wc.GetAllProducts(cnf, 10)

	errEnd := make(chan struct{}, 0)
	go func() {
		defer close(errEnd)
		for err := range errors {
			fmt.Println(err)
		}
	}()

	fmt.Println("Reading products from WC site...")
	for product := range products {
		delete(createCache, product.SKU)
		if _, ok := lookup[product.SKU]; !ok {
			deleteList = append(deleteList, product.ID)
		}
	}

	<-errEnd

	fmt.Println("Deleting products that weren't on Tarsus...")
	errors = wc.DeleteProducts(cnf, deleteList, 3, 40)
	for err := range errors {
		fmt.Println(err)
	}

	createProducts := make([]types.WooCommerceProduct, len(createCache))
	i := 0
	for sku := range createCache {
		tarsusProduct := lookup[sku]
		wcProduct := wc.FromTarsusProduct(tarsusProduct)
		createProducts[i] = wcProduct
		i++
	}

	if bytes, err := json.Marshal(createProducts); err == nil {
		fmt.Println(string(bytes))
	}

	time.Sleep(time.Second)

	errors = wc.CreateProducts(cnf, createProducts, 1)
	for err := range errors {
		fmt.Fprintln(os.Stderr, err)
	}
}
