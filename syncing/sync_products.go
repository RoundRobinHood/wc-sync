package syncing

import (
	"fmt"
	"os"
	"time"

	"github.com/RoundRobinHood/jouma-data-migration/types"
	"github.com/RoundRobinHood/jouma-data-migration/wc"
	"github.com/cheggaaa/pb/v3"
)

func SyncProducts(wp_cnf, wc_cnf types.ApiConfig, TarsusProducts []types.TarsusProduct) {
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

	products, errors := wc.GetAllProducts(wc_cnf, 10)

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
		errors = wc.DeleteProducts(wc_cnf, deleteList, 3, 40)
		for err := range errors {
			fmt.Println(err)
		}
	}

	if len(createCache) == 0 {
		fmt.Println("No products to be created.")
	} else {
		SKUs := make([]string, 0, len(createCache))
		for sku := range createCache {
			SKUs = append(SKUs, sku)
		}
		fmt.Println("SKUs to be created:", SKUs)

		fmt.Println("Validating & converting Tarsus products to WooCommerce products...")
		bar := pb.StartNew(len(createCache))
		createProducts := make([]types.WooCommerceProduct, 0, len(createCache))
		for sku := range createCache {
			exists, err := wc.SKUExists(wc_cnf, sku)
			if err != nil {
				fmt.Printf("Failed to check if product already exists (skipping product, SKU %q): \n%v\n", sku, err)
				time.Sleep(time.Second)
				bar.Increment()
				continue
			}
			if exists {
				fmt.Println("Product SKU already exists on WP site. Skipping")
				time.Sleep(time.Second)
				bar.Increment()
				continue
			}

			tarsusProduct := lookup[sku]
			time.Sleep(time.Second)
			wcProduct, err := wc.FromTarsusProduct(tarsusProduct, wp_cnf)
			if err != nil {
				fmt.Printf("Failed to convert Tarsus Product (SKU: %q): %v\n", sku, err)
			} else {
				createProducts = append(createProducts, wcProduct)
			}
			if len(wcProduct.Images) == 0 {
				fmt.Printf("WARNING: Product (SKU: %q) had an invalid image and is scheduled to be created with no images.\n", sku)
			}
			bar.Increment()
			time.Sleep(time.Second)
		}
		bar.Finish()

		time.Sleep(time.Second)

		if len(createProducts) == 0 {
			fmt.Println("No product creation required.")
		} else {
			fmt.Println("Creating products that weren't on the WP site...")
			errors = wc.CreateProducts(wp_cnf, wc_cnf, createProducts, 1)
			for err := range errors {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
}
