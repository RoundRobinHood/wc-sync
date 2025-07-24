package wc

import (
	"fmt"
	"sync"
	"time"

	"github.com/RoundRobinHood/jouma-data-migration/rest"
	"github.com/RoundRobinHood/jouma-data-migration/types"
	"github.com/cheggaaa/pb/v3"
)

var CategoriesPerRequest = 100

func GetAllCategories(WCCnf types.ApiConfig, workerCount int) (chan types.WCCategory, chan error) {
	infoUrl := WCCnf.BaseUrl + "/wp-json/wc/v3/products/categories?per_page=1&orderby=id&order=asc&_=" + fmt.Sprint(time.Now().UnixMilli())

	categories, errors := make(chan types.WCCategory, 0), make(chan error, 0)

	go func() {
		category_count, err := GetItemCount(infoUrl, &rest.RequestOptions{
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
			close(categories)
			return
		}

		pageCount := (category_count + CategoriesPerRequest - 1) / CategoriesPerRequest
		bar := pb.StartNew(pageCount)
		defer bar.Finish()
		defer close(errors)
		defer close(categories)

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
					url := fmt.Sprintf("%s/wp-json/wc/v3/products/categories?per_page=%d&page=%d&orderby=id&order=asc&_=%d", WCCnf.BaseUrl, CategoriesPerRequest, page, time.Now().UnixMilli())
					var response_categories []types.WCCategory
					resp, err := wc_client.Request(url, &rest.RequestOptions{
						Method:           "GET",
						Headers:          map[string]string{"Authorization": "Basic " + WCCnf.APIKey},
						WithNetworkRetry: true,
						RetryDelay:       time.Second,
					}, &response_categories)
					if err != nil {
						if resp.StatusCode == 429 {
							fmt.Printf("Worker %d retrying: 429\n", i)
							jitterSleep(true)
							goto retry
						}
						errors <- fmt.Errorf("Failed to fetch %q (klling worker %d): %w", url, i, err)
						return
					}
					if resp.StatusCode != 200 {
						errors <- fmt.Errorf("Wrong status code %d for %q (killing worker %d)", resp.StatusCode, url, i)
						return
					}
					for _, category := range response_categories {
						categories <- category
					}
					bar.Increment()
					jitterSleep(false)
				}
			}(i)
		}

		wg.Wait()
	}()

	return categories, errors
}

func DeleteCategories(WCCnf types.ApiConfig, IDs []int, workerCount, maxBatch int) chan error {
	batchChannel := make(chan []int, 0)
	go func() {
		for i := 0; i < len(IDs); i += maxBatch {
			batchChannel <- IDs[i:min(i+maxBatch, len(IDs))]
		}
		close(batchChannel)
	}()

	errors := make(chan error, 0)

	go func() {
		url := WCCnf.BaseUrl + "/wp-json/wc/v3/products/categories/batch"
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
						errors <- fmt.Errorf("unexpected status code %d (killing worker %d). Response body: \n%s\n", resp.StatusCode, i, string(resp.Body))
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

func CreateCategories(WCCnf types.ApiConfig, Categories []types.WCCategory, workerCount, maxBatch int) chan error {
	batchChannel := make(chan []types.WCCategory, 0)
	go func() {
		for i := 0; i < len(Categories); i += maxBatch {
			batchChannel <- Categories[i:min(i+maxBatch, len(Categories))]
		}
		close(batchChannel)
	}()

	errors := make(chan error, 0)

	go func() {
		url := WCCnf.BaseUrl + "/wp-json/wc/v3/products/categories/batch"
		wg := new(sync.WaitGroup)
		wg.Add(workerCount)
		bar := pb.StartNew(len(Categories))
		defer bar.Finish()
		defer close(errors)

		for i := range workerCount {
			go func(i int) {
				defer wg.Done()
				for categories := range batchChannel {
				batch_start:
					resp, err := wc_client.Request(url, &rest.RequestOptions{
						Method:           "POST",
						Headers:          map[string]string{"Authorization": "Basic " + WCCnf.APIKey},
						Body:             map[string]any{"create": categories},
						WithNetworkRetry: true,
						RetryDelay:       time.Second,
					}, nil)

					if err != nil {
						errors <- fmt.Errorf("create batch failed (killing worker %d)", i)
						return
					}

					if resp.StatusCode != 200 {
						if resp.StatusCode == 429 {
							errors <- fmt.Errorf("worker %d wait-retrying", i)
							jitterSleep(true)
							goto batch_start
						}
						errors <- fmt.Errorf("unexpected status code %d (killing worker %d). Response body: \n%s", resp.StatusCode, i, string(resp.Body))
						return
					}

					bar.Add(len(categories))
					jitterSleep(false)
				}
			}(i)
		}
		wg.Wait()
	}()

	return errors
}
