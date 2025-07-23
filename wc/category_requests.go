package wc

import (
	"fmt"

	"github.com/RoundRobinHood/jouma-data-migration/rest"
	"github.com/RoundRobinHood/jouma-data-migration/types"
)

func GetAllCategories(WCCnf types.ApiConfig) ([]types.WCCategory, error) {
	shouldRetry := true
	var categories []types.WCCategory
	resp, err := wc_client.Request(WCCnf.BaseUrl+"/wp-json/wc/v3/products/categories?per_page=100", &rest.RequestOptions{
		Method:           "GET",
		Headers:          map[string]string{"Authorization": "Basic " + WCCnf.APIKey},
		WithNetworkRetry: shouldRetry,
	}, &categories)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Unexpected StatusCode from retrieving categories: %d", resp.StatusCode)
	}

	return categories, nil
}
