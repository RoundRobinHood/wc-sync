package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/RoundRobinHood/jouma-data-migration/rest"
	"github.com/RoundRobinHood/jouma-data-migration/syncing"
	"github.com/RoundRobinHood/jouma-data-migration/types"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	source := flag.String("source", "file", "Source of the data: 'api' or 'file'")
	filePath := flag.String("file", "data.json", "Path to JSON file if using file source")
	tarsusURL := flag.String("api", "https://feedgen.tarsusonline.co.za/api/DataFeed/Customer-ProductCatalogue", "URL to GET for Tarsus Products")
	flag.Parse()

	key := ""
	secret := ""
	wc_url := ""
	tarsus_key := ""
	app_user := ""
	app_pass := ""

	if key = os.Getenv("WC_CONSUMER_KEY"); key == "" {
		fmt.Fprintln(os.Stderr, "Please provide WC consumer key")
		return
	}
	if secret = os.Getenv("WC_CONSUMER_SECRET"); secret == "" {
		fmt.Fprintln(os.Stderr, "Please provide WC consumer secret")
		return
	}
	if wc_url = os.Getenv("APP_URL"); wc_url == "" {
		fmt.Fprintln(os.Stderr, "Please provide WP site url")
		return
	}
	if tarsus_key = os.Getenv("TARSUS_KEY"); tarsus_key == "" {
		fmt.Fprintln(os.Stderr, "Please provide Tarsus API key")
		return
	}
	if app_user = os.Getenv("APP_USER"); app_user == "" {
		fmt.Fprintln(os.Stderr, "Please provide application username")
		return
	}
	if app_pass = os.Getenv("APP_PWD"); app_pass == "" {
		fmt.Fprintln(os.Stderr, "Please provide application password")
		return
	}

	var bytes []byte
	fmt.Println("Getting tarsus products...")
	if strings.ToLower(*source) == "api" {
		resp, err := rest.Request(*tarsusURL, &rest.RequestOptions{
			Method:           "GET",
			Headers:          map[string]string{"Authorization": "Bearer " + tarsus_key},
			WithNetworkRetry: true,
		}, nil, nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to GET tarsusUrl:", err)
			return
		}
		bytes = resp.Body
		err = os.WriteFile(*filePath, bytes, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to back up tarsus data to %s before processing. Err: \n%s\n", *filePath, err)
		}
		fmt.Printf("Products aqcuired from %q\n", *tarsusURL)
	} else {
		var err error
		bytes, err = os.ReadFile(*filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read tarsus products from %q. Err: \n%s\n", *filePath, err)
			return
		}
		fmt.Printf("Products acquired from %q\n", *filePath)
	}

	var products struct {
		Products []types.TarsusProduct `json:"products"`
	}
	if err := json.Unmarshal(bytes, &products); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to unmarshal Tarsus products:", err)
		return
	}

	basic := base64.StdEncoding.EncodeToString([]byte(key + ":" + secret))
	wc_config := types.ApiConfig{
		BaseUrl: wc_url,
		APIKey:  basic,
	}

	fmt.Printf("Syncing towards WooCommerce API at %q\n", wc_url)
	syncing.SyncUp(wc_config, products.Products)
}
