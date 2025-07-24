package wc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/RoundRobinHood/jouma-data-migration/rest"
	"github.com/RoundRobinHood/jouma-data-migration/types"
	"github.com/RoundRobinHood/jouma-data-migration/wp"
	"github.com/cheggaaa/pb/v3"
)

var wc_client = rest.RestClient{
	Client: &http.Client{},
}

func jitterSleep(rateLimited bool) {
	jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
	if rateLimited {
		jitter *= 4
	}
	time.Sleep(time.Second + jitter)
}

func GetItemCount(url string, opt *rest.RequestOptions) (int, error) {
func_start:
	resp, err := wc_client.Request(url, opt, nil)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != 200 {
		if resp.StatusCode == 429 {
			jitterSleep(true)
			goto func_start
		}
		return 0, fmt.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	total_header := resp.Header.Get("X-WP-Total")
	var total int
	_, err = fmt.Sscan(total_header, &total)
	if err != nil {
		return 0, fmt.Errorf("Invalid product count header: %w", err)
	}

	return total, nil
}

func SKUExists(WCCnf types.ApiConfig, SKU string) (bool, error) {
func_start:
	var products []types.WooCommerceProduct
	resp, err := wc_client.Request(WCCnf.BaseUrl+"/wp-json/wc/v3/products?sku="+url.QueryEscape(SKU), &rest.RequestOptions{
		Method:           "GET",
		Headers:          map[string]string{"Authorization": "Basic " + WCCnf.APIKey},
		WithNetworkRetry: true,
	}, &products)
	if err != nil {
		if resp.StatusCode == 429 {
			fmt.Printf("Got 429 when checking for SKU (%q). Retrying...\n", SKU)
			jitterSleep(true)
			goto func_start
		}
		return false, fmt.Errorf("failed to check for sku: %w", err)
	}

	if resp.StatusCode != 200 {
		return false, fmt.Errorf("unexpected statuscode: %d", resp.StatusCode)
	}

	return len(products) != 0, nil
}

func CreateProduct(WPCnf types.ApiConfig, WCCnf types.ApiConfig, product types.WooCommerceProduct) error {
func_start:
	fmt.Printf("Attempting to create product with SKU %q\n", product.SKU)
	resp, err := wc_client.Request(WCCnf.BaseUrl+"/wp-json/wc/v3/products", &rest.RequestOptions{
		Method:           "POST",
		Headers:          map[string]string{"Authorization": "Basic " + WCCnf.APIKey},
		Body:             product,
		WithNetworkRetry: true,
	}, nil)

	if err != nil {
		return err
	}

	if resp.StatusCode != 201 {

		if resp.StatusCode == 504 {
			fmt.Println("Got 504.")
			fmt.Println("Sleeping & checking if image made it onto the server...")
			jitterSleep(false)
			if len(product.Images) != 0 && product.Images[0].Href != "" {
				id, err := wp.GetImageID(WPCnf, product.Images[0].Href)
				if err != nil {
					if !errors.Is(err, wp.ErrImageNotExist) {
						return fmt.Errorf("failed to check if image exists: %w", err)
					} else {
						fmt.Println("Image not on server. Retrying 504 again.")
					}
				} else {
					fmt.Println("Image already on server. Retrying with its ID...")
					product.Images[0] = types.WCImage{Id: id}
				}
			}
			jitterSleep(true)
			goto func_start
		}

		if resp.StatusCode == 429 {
			fmt.Printf("Retrying product creation (SKU: %q)\n", product.SKU)
			jitterSleep(true)
			goto func_start
		}
		if resp.StatusCode == 400 {
			var errResponse struct {
				Code string `json:"code"`
			}
			fmt.Printf("Interpreting 400 response for product (SKU %q)\n", product.SKU)
			if err := json.Unmarshal(resp.Body, &errResponse); err == nil {
				if errResponse.Code == "woocommerce_product_image_upload_error" {
					fmt.Fprintf(os.Stderr, "WARNING: Image error for product (SKU: %q). Resending POST without images\n", product.SKU)
					product.Images = []types.WCImage{}
					goto func_start
				} else if errResponse.Code == "product_invalid_sku" {
					fmt.Printf("Invalid sku (%q), checking if product already exists...\n", product.SKU)
					exists, err := SKUExists(WCCnf, product.SKU)
					if err != nil {
						return fmt.Errorf("failed to double-check if product (SKU: %q) already exists: %w", product.SKU, err)
					}

					if exists {
						return nil
					} else {
						return fmt.Errorf("invalid SKU (%q). Response body:\n%s\n", product.SKU, string(resp.Body))
					}
				}
			}
		}
		return fmt.Errorf("Unexpected status code: %d\nResponse body:\n%s\n", resp.StatusCode, string(resp.Body))
	}
	return nil
}

func UpdateProduct(WCCnf types.ApiConfig, product types.WooCommerceProduct) error {
	url := WCCnf.BaseUrl + "/wp-json/wc/v3/products/" + fmt.Sprint(product.ID)
	product.ID = 0
	resp, err := wc_client.Request(url, &rest.RequestOptions{
		Method:  "PUT",
		Headers: map[string]string{"Authorization": "Basic " + WCCnf.APIKey},
		Body:    product,
	}, nil)

	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("Unexpected status code: %d\nResponse body:\n%s\n", resp.StatusCode, string(resp.Body))
	}

	return nil
}

var ProductsPerRequest = 100

func GetAllProducts(WCCnf types.ApiConfig, workerCount int) (chan types.WooCommerceProduct, chan error) {
	infoUrl := WCCnf.BaseUrl + "/wp-json/wc/v3/products?per_page=1&orderby=id&order=asc&_=" + fmt.Sprint(time.Now().UnixMilli())
	products, errors := make(chan types.WooCommerceProduct, 0), make(chan error, 0)

	go func() {
		product_count, err := GetItemCount(infoUrl, &rest.RequestOptions{
			Method:           "GET",
			WithNetworkRetry: true,
			RetryDelay:       time.Second,
			Headers:          map[string]string{"Authorization": "Basic " + WCCnf.APIKey},
		})
		if err != nil {
			go func() {
				errors <- fmt.Errorf("Failed info request: %w", err)
				close(errors)
			}()
			close(products)
			return
		}

		pageCount := (product_count + ProductsPerRequest - 1) / ProductsPerRequest
		bar := pb.StartNew(pageCount)
		defer bar.Finish()
		defer close(errors)
		defer close(products)

		pageChannel := make(chan int, 0)
		go func() {
			for i := range pageCount {
				pageChannel <- i + 1
			}
			close(pageChannel)
		}()

		wg := new(sync.WaitGroup)
		wg.Add(workerCount)

		jitterSleep(false)
		for i := range workerCount {
			go func(i int) {
				defer wg.Done()
				for page := range pageChannel {
				retry:
					url := fmt.Sprintf("%s/wp-json/wc/v3/products?per_page=%d&page=%d&orderby=id&order=asc&_=%d", WCCnf.BaseUrl, ProductsPerRequest, page, time.Now().UnixMilli())
					var response_products []types.WooCommerceProduct
					resp, err := wc_client.Request(url, &rest.RequestOptions{
						Method:           "GET",
						Headers:          map[string]string{"Authorization": "Basic " + WCCnf.APIKey},
						WithNetworkRetry: true,
						RetryDelay:       time.Second,
					}, &response_products)
					if err != nil {
						if resp.StatusCode == 429 {
							fmt.Printf("Worker %d retrying: 429\n", i)
							jitterSleep(true)
							goto retry
						}
						errors <- fmt.Errorf("Failed to fetch %q (killing worker %d): %w", url, i, err)
						return
					}
					if resp.StatusCode != 200 {
						errors <- fmt.Errorf("Wrong status code %d for product fetch URL %q, killing worker %d", resp.StatusCode, url, i)
						return
					}
					for _, product := range response_products {
						products <- product
					}
					bar.Increment()
					jitterSleep(false)
				}
			}(i)
		}

		wg.Wait()
	}()

	return products, errors
}

var TagsPerRequest = 100

func GetAllTags(WCCnf types.ApiConfig, workerCount int) (chan types.WCTag, chan error) {
	infoUrl := WCCnf.BaseUrl + "/wp-json/wc/v3/products/tags?per_page=1"
	tags, errors := make(chan types.WCTag, 0), make(chan error, 0)

	go func() {
		tag_count, err := GetItemCount(infoUrl, &rest.RequestOptions{
			Method:           "GET",
			WithNetworkRetry: true,
			Headers:          map[string]string{"Authorization": "Basic " + WCCnf.APIKey},
		})
		if err != nil {
			go func() {
				errors <- fmt.Errorf("Failed info request: %w", err)
				close(errors)
			}()
			close(tags)
			return
		}

		pageCount := (tag_count + TagsPerRequest - 1) / TagsPerRequest

		pageChannel := make(chan int, 0)
		go func() {
			for i := range pageCount {
				pageChannel <- i + 1
			}
			close(pageChannel)
		}()

		wg := new(sync.WaitGroup)
		wg.Add(workerCount)

		for i := range workerCount {
			go func(i int) {
				defer wg.Done()
				for page := range pageChannel {
				retry:
					url := fmt.Sprintf("%s/wp-json/wc/v3/products/tags?per_page=%d&page=%d", WCCnf.BaseUrl, TagsPerRequest, page)
					var response_tags []types.WCTag
					resp, err := wc_client.Request(url, &rest.RequestOptions{
						Method:           "GET",
						Headers:          map[string]string{"Authorization": "Basic " + WCCnf.APIKey},
						WithNetworkRetry: true,
					}, &response_tags)
					if err != nil {
						errors <- fmt.Errorf("Failed to fetch %q (killing worker %d): %w", url, i, err)
						return
					}
					if resp.StatusCode != 200 {
						if resp.StatusCode == 409 {
							jitterSleep(true)
							goto retry
						}
						errors <- fmt.Errorf("Wrong status code %d for %q (killing worker %d)", resp.StatusCode, url, i)
						return
					}
					for _, tag := range response_tags {
						tags <- tag
					}
				}
			}(i)
		}

		wg.Wait()
		close(tags)
		close(errors)
	}()

	return tags, errors
}

func CreateProducts(WPCnf, WCCnf types.ApiConfig, Products []types.WooCommerceProduct, workerCount int) chan error {
	fmt.Printf("Creating %d products with %d workers", len(Products), workerCount)
	productChannel := make(chan types.WooCommerceProduct, 0)
	go func() {
		for _, product := range Products {
			productChannel <- product
		}
		close(productChannel)
	}()

	errors := make(chan error, 0)

	go func() {
		wg := new(sync.WaitGroup)
		wg.Add(workerCount)
		bar := pb.StartNew(len(Products))
		defer bar.Finish()
		for i := range workerCount {
			go func(i int) {
				defer wg.Done()
				for product := range productChannel {
					if err := CreateProduct(WPCnf, WCCnf, product); err != nil {
						errors <- err
						return
					}
					bar.Increment()
					jitterSleep(false)
				}
			}(i)
		}

		wg.Wait()
		close(errors)
	}()

	return errors
}

func DeleteProducts(WCCnf types.ApiConfig, IDs []int, workerCount, maxBatch int) chan error {
	batchChannel := make(chan []int, 0)
	go func() {
		for i := 0; i < len(IDs); i += maxBatch {
			batchChannel <- IDs[i:min(i+maxBatch, len(IDs))]
		}
		close(batchChannel)
	}()

	errors := make(chan error, 0)

	go func() {
		url := WCCnf.BaseUrl + "/wp-json/wc/v3/products/batch"
		wg := new(sync.WaitGroup)
		wg.Add(workerCount)
		bar := pb.StartNew(len(IDs))
		defer bar.Finish()
		defer close(errors)
		for i := range workerCount {
			go func(i int) {
				defer wg.Done()
				for ids := range batchChannel {
				batch_start:
					resp, err := wc_client.Request(url, &rest.RequestOptions{
						Method:           "POST",
						Headers:          map[string]string{"Authorization": "Basic " + WCCnf.APIKey},
						Body:             map[string]any{"delete": ids},
						WithNetworkRetry: true,
						RetryDelay:       time.Second,
					}, nil)

					if err != nil {
						errors <- fmt.Errorf("delete request failed (killing worker %d)", i)
						return
					}

					if resp.StatusCode != 200 {
						if resp.StatusCode == 429 {
							errors <- fmt.Errorf("worker %d wait-retrying", i)
							jitterSleep(true)
							goto batch_start
						}
						errors <- fmt.Errorf("unexpected statuscode %d (killing worker %d). Response body: \n%s\n", resp.StatusCode, i, string(resp.Body))
						return
					}

					bar.Add(len(ids))
					jitterSleep(false)
				}
			}(i)
		}
		wg.Wait()
	}()

	return errors
}
