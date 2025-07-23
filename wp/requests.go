package wp

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/RoundRobinHood/jouma-data-migration/rest"
	"github.com/RoundRobinHood/jouma-data-migration/types"
)

var wp_client = &rest.RestClient{
	Client: &http.Client{},
}

var ErrImageNotExist = errors.New("image does not exist")

func GetImageID(WPCnf types.ApiConfig, source string) (int, error) {
	sleep_seconds := 1
func_start:
	search := source
	if strings.Contains(source, "/") {
		fileURL, err := url.Parse(source)
		if err != nil {
			return 0, err
		}
		search = path.Base(fileURL.Path)
	}
	search = strings.TrimSuffix(search, path.Ext(search))

	var response []types.WPImage
	url := fmt.Sprintf("%s/wp-json/wp/v2/media?search=%s", WPCnf.BaseUrl, url.QueryEscape(search))
	resp, err := wp_client.Request(url, &rest.RequestOptions{
		Method:           "GET",
		Headers:          map[string]string{"Authorization": "Basic " + WPCnf.APIKey},
		WithNetworkRetry: true,
		RetryDelay:       time.Second,
	}, &response)
	if err != nil {
		if resp.StatusCode == 429 {
			fmt.Printf("429 received from media request. Sleeping for %d seconds...\n", sleep_seconds)
			time.Sleep(time.Second * time.Duration(sleep_seconds))
			sleep_seconds *= 2
			goto func_start
		}
		return 0, err
	}

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if len(response) == 0 {
		return 0, ErrImageNotExist
	}

	return response[0].ID, nil
}
