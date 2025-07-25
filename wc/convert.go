package wc

import (
	"errors"
	"fmt"
	"math"

	"github.com/RoundRobinHood/jouma-data-migration/rest"
	"github.com/RoundRobinHood/jouma-data-migration/types"
	"github.com/RoundRobinHood/jouma-data-migration/wp"
)

func FromTarsusProduct(product types.TarsusProduct, WPCnf types.ApiConfig) (types.WooCommerceProduct, error) {
	ret := types.WooCommerceProduct{
		SKU:         product.ProductNumber,
		Name:        product.ShortDesc,
		Description: product.Description,
		Tags: []types.WCTag{
			{Name: product.ProductType},
			{Name: product.Manufacturer},
		},
		Categories: []types.WCCategory{
			{Name: product.Category},
		},
		StockQtty:    product.Stock,
		RegularPrice: string(product.PriceExVAT),
		Images:       make([]types.WCImage, 0),
		Dimensions: &types.WCDimensions{
			Width:  fmt.Sprint(product.Width),
			Height: fmt.Sprint(product.Height),
		},
		Weight: fmt.Sprint(product.Weight),
	}
	if product.ImageURL != "" {
		resp, err := rest.Request(product.ImageURL, &rest.RequestOptions{Method: "HEAD", WithNetworkRetry: true}, nil, nil)
		if err != nil {
			return ret, fmt.Errorf("failed to verify image URL: %w", err)
		}
		if resp.StatusCode == 200 {
			id, err := wp.GetImageID(WPCnf, product.ImageURL)
			if err != nil {
				if errors.Is(err, wp.ErrImageNotExist) {
					ret.Images = append(ret.Images, types.WCImage{Href: product.ImageURL})
				} else {
					return ret, fmt.Errorf("failed to check for image existence on WP: %w", err)
				}
			} else {
				ret.Images = append(ret.Images, types.WCImage{Id: id})
			}
		}
	}
	return ret, nil
}

func ConvertEquals(wc types.WooCommerceProduct, ts types.TarsusProduct) bool {
	// Exact eq strings
	if wc.SKU != ts.ProductNumber || wc.Name != ts.ShortDesc ||
		wc.Description != ts.Description || wc.StockQtty != ts.Stock {
		return false
	}

	var wc_regular_price, ts_price_exVat, wc_width, wc_height, wc_length, wc_weight float64
	fmt.Sscan(wc.RegularPrice, &wc_regular_price)
	fmt.Sscan(string(ts.PriceExVAT), &ts_price_exVat)
	fmt.Sscan(wc.Dimensions.Width, &wc_width)
	fmt.Sscan(wc.Dimensions.Height, &wc_height)
	fmt.Sscan(wc.Dimensions.Length, &wc_length)
	fmt.Sscan(wc.Weight, &wc_weight)

	cmp_epsilon := float64(0.00001)
	// Exact eq floats
	if wc_regular_price != ts_price_exVat || math.Abs(wc_weight-ts.Weight) > cmp_epsilon ||
		math.Abs(wc_length-ts.Length) > cmp_epsilon || math.Abs(wc_width-ts.Width) > cmp_epsilon ||
		math.Abs(wc_height-ts.Height) > cmp_epsilon {
		return false
	}

	if len(wc.Tags) != 2 {
		return false
	}

	hasProductType, hasManufacturer := false, false
	for _, tag := range wc.Tags {
		if tag.Name == ts.ProductType {
			hasProductType = true
		}
		if tag.Name == ts.Manufacturer {
			hasManufacturer = true
		}
	}

	if !hasProductType || !hasManufacturer {
		return false
	}

	if len(wc.Images) != 1 || wc.Images[0].Href != ts.ImageURL {
		return false
	}

	if len(wc.Categories) != 1 || wc.Categories[0].Name != ts.Category {
		return false
	}

	return true
}
