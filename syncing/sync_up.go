package syncing

import (
	"fmt"
	"os"
	"time"

	"github.com/RoundRobinHood/jouma-data-migration/types"
	"github.com/RoundRobinHood/jouma-data-migration/wc"
	"github.com/cheggaaa/pb/v3"
)

func SyncUp(wp_cnf, cnf types.ApiConfig, TarsusProducts []types.TarsusProduct) {
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

	if len(deleteList) == 0 {
		fmt.Println("No products to delete on WP site.")
	} else {
		fmt.Println("Deleting products that weren't on Tarsus...")
		errors = wc.DeleteProducts(cnf, deleteList, 3, 40)
		for err := range errors {
			fmt.Println(err)
		}
	}

	fmt.Println("Converting Tarsus products to WooCommerce products...")
	bar := pb.StartNew(len(createCache))
	createProducts := make([]types.WooCommerceProduct, 0, len(createCache))
	for sku := range createCache {
		tarsusProduct := lookup[sku]
		wcProduct, err := wc.FromTarsusProduct(tarsusProduct, wp_cnf)
		if err != nil {
			fmt.Printf("Failed to convert Tarsus Product (SKU: %q): %v\n", sku, err)
		} else {
			createProducts = append(createProducts, wcProduct)
		}
		bar.Increment()
		time.Sleep(time.Second)
	}
	bar.Finish()

	time.Sleep(time.Second)

	fmt.Println("Creating products that weren't on the WP site...")
	errors = wc.CreateProducts(wp_cnf, cnf, createProducts, 1)
	for err := range errors {
		fmt.Fprintln(os.Stderr, err)
	}
}
