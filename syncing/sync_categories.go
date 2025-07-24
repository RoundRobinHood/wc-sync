package syncing

import (
	"fmt"

	"github.com/RoundRobinHood/jouma-data-migration/types"
	"github.com/RoundRobinHood/jouma-data-migration/wc"
)

func SyncCategories(wc_cnf types.ApiConfig, TarsusProducts []types.TarsusProduct) {
	// Used to quickly check SKUs against tarsus products
	lookup := map[string]*types.WCCategory{}

	// Delete a key when you find it (leftovers have to be created on WC)
	createCache := map[string]struct{}{}

	// List of IDs to be deleted
	deleteList := make([]int, 0)

	for _, product := range TarsusProducts {
		lookup[product.Category] = &types.WCCategory{
			Name: product.Category,
		}
		lookup[product.ProductType] = &types.WCCategory{
			Name:       product.ProductType,
			ParentName: product.Category,
		}
		createCache[product.Category] = struct{}{}
		createCache[product.ProductType] = struct{}{}
	}

	categories, errors := wc.GetAllCategories(wc_cnf, 10)

	errEnd := make(chan struct{}, 0)
	go func() {
		defer close(errEnd)
		for err := range errors {
			fmt.Println(err)
		}
	}()

	fmt.Println("Reading category pages from WC...")
	for category := range categories {
		if ptr := lookup[category.Name]; ptr != nil {
			ptr.Id = category.Id
			ptr.ParentID = category.ParentID
			for _, c := range lookup {
				if c.ParentName == ptr.Name {
					c.ParentID = ptr.Id
				}
			}
		}
		delete(createCache, category.Name)
		if _, ok := lookup[category.Name]; !ok {
			deleteList = append(deleteList, category.Id)
		}
	}

	<-errEnd
	if len(deleteList) == 0 {
		fmt.Println("No categories to delete on WC.")
	} else {
		fmt.Println("Deleting categories...")
		errors := wc.DeleteCategories(wc_cnf, deleteList, 3, 40)
		for err := range errors {
			fmt.Println(err)
		}
	}

	if len(createCache) == 0 {
		fmt.Println("No categories to create")
	} else {
		workerCount := 4
		batchSize := 10

		fmt.Printf("Creating categories with %d workers and size %d batches\n", workerCount, batchSize)

		createList := make([]types.WCCategory, 0, len(createCache))
		for name := range createCache {
			createList = append(createList, *lookup[name])
		}

		errors := wc.CreateCategories(wc_cnf, createList, workerCount, batchSize)
		for err := range errors {
			fmt.Println(err)
		}
	}
}
