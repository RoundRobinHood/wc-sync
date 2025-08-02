package wc

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"time"

	"github.com/RoundRobinHood/jouma-data-migration/rest"
	"github.com/RoundRobinHood/jouma-data-migration/types"
	"github.com/RoundRobinHood/jouma-data-migration/wp"
)

func ValidateImage(imageURL string, WPCnf types.ApiConfig) (*types.WCImage, error) {
	if imageURL == "" {
		return nil, nil
	}
	resp, err := rest.Request(imageURL, &rest.RequestOptions{
		Method:           "HEAD",
		WithNetworkRetry: true,
	}, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to verify image URL: %w", err)
	}
	if resp.StatusCode == 200 {
		id, err := wp.GetImageID(WPCnf, imageURL)
		if err != nil {
			if errors.Is(err, wp.ErrImageNotExist) {
				return &types.WCImage{Href: imageURL}, nil
			} else {
				return nil, fmt.Errorf("failed to check image existence on WP: %w", err)
			}
		}
		time.Sleep(time.Second)
		return &types.WCImage{Id: id}, nil
	}

	return nil, nil
}

func FromTarsusProduct(product types.TarsusProduct, WPCnf types.ApiConfig, category_ids map[string]int) (types.WooCommerceProduct, error) {
	st := product.Stock
	categories := make([]types.WCCategory, 0, 1)
	if id, ok := category_ids[product.ProductType]; ok {
		categories = append(categories, types.WCCategory{Id: id})
	} else {
		fmt.Printf("WARNING: Product (SKU: %q) needs a category (%q) with an unknown ID\n", product.ProductNumber, product.ProductType)
		categories = append(categories, types.WCCategory{Name: product.ProductType})
	}

	ret := types.WooCommerceProduct{
		SKU:         product.ProductNumber,
		Name:        product.ShortDesc,
		Description: product.Description,
		Tags: []types.WCTag{
			{Name: product.Manufacturer},
		},
		Categories:   categories,
		StockQtty:    &st,
		RegularPrice: string(product.PriceExVAT),
		Images:       make([]types.WCImage, 0),
		Dimensions: &types.WCDimensions{
			Width:  fmt.Sprint(product.Width),
			Height: fmt.Sprint(product.Height),
			Length: fmt.Sprint(product.Length),
		},
		Weight: fmt.Sprint(product.Weight),
	}

	img, err := ValidateImage(product.ImageURL, WPCnf)
	if err != nil {
		return ret, err
	}

	if img != nil {
		ret.Images = append(ret.Images, *img)
	}

	return ret, nil
}

func MakeUpdate(latest types.TarsusProduct, current types.WooCommerceProduct, WPCnf types.ApiConfig, category_ids map[string]int) (*types.WooCommerceProduct, error) {
	ret := types.WooCommerceProduct{}
	if current.SKU != latest.ProductNumber {
		ret.SKU = latest.ProductNumber
	}
	if current.Name != latest.ShortDesc {
		ret.Name = latest.ShortDesc
	}

	hasManufacturer := false
	for _, tag := range current.Tags {
		if tag.Name == latest.Manufacturer {
			hasManufacturer = true
		}
	}
	if !hasManufacturer {
		tags := make([]types.WCTag, len(current.Tags), len(current.Tags)+1)
		copy(tags, current.Tags)
		tags = append(tags, types.WCTag{Name: latest.Manufacturer})
	}
	hasType := false
	type_id, found := category_ids[latest.ProductType]
	if !found {
		fmt.Printf("WARNING: Category (%q) has unknown ID\n", latest.ProductType)
	}

	for _, category := range current.Categories {
		if (category.Id == type_id && found) || category.Name == latest.ProductType {
			hasType = true
		}
	}
	if !hasType {
		if found {
			ret.Categories = []types.WCCategory{{Id: type_id}}
		} else {
			ret.Categories = []types.WCCategory{{Name: latest.ProductType}}
		}
	}

	if current.StockQtty == nil || *current.StockQtty != latest.Stock {
		st := latest.Stock
		current.StockQtty = &st
	}
	if current.RegularPrice != string(latest.PriceExVAT) {
		var a, b float64
		if _, err := fmt.Sscan(current.RegularPrice, &a); err != nil {
			ret.RegularPrice = string(latest.PriceExVAT)
		} else {
			fmt.Sscan(string(latest.PriceExVAT), &b)
			if math.Abs(a-b) >= 0.001 {
				ret.RegularPrice = string(latest.PriceExVAT)
			}
		}
	}
	img, err := ValidateImage(latest.ImageURL, WPCnf)
	if err != nil {
		return nil, err
	}

	if img != nil {
		if img.Id == 0 {
			images := make([]types.WCImage, len(current.Images))
			copy(images, current.Images)
			ret.Images = append(images, *img)
		} else {
			hasImg := false
			for _, image := range current.Images {
				if image.Id == img.Id {
					hasImg = true
				}
			}
			if !hasImg {
				images := make([]types.WCImage, len(current.Images))
				copy(images, current.Images)
				ret.Images = append(images, *img)
			}
		}
	}

	if current.Dimensions == nil {
		ret.Dimensions = &types.WCDimensions{
			Width:  fmt.Sprint(latest.Width),
			Height: fmt.Sprint(latest.Height),
			Length: fmt.Sprint(latest.Length),
		}
	} else {
		newDim := types.WCDimensions{}
		if current.Dimensions.Width == "" {
			newDim.Width = fmt.Sprint(latest.Width)
		} else {
			var width float64
			if _, err := fmt.Sscan(current.Dimensions.Width, &width); err != nil {
				newDim.Width = fmt.Sprint(latest.Width)
			} else if math.Abs(width-latest.Width) > 0.001 {
				newDim.Width = fmt.Sprint(latest.Width)
			}
		}
		if current.Dimensions.Height == "" {
			newDim.Height = fmt.Sprint(latest.Height)
		} else {
			var height float64
			if _, err := fmt.Sscan(current.Dimensions.Height, &height); err != nil {
				newDim.Height = fmt.Sprint(latest.Height)
			} else if math.Abs(height-latest.Height) > 0.001 {
				newDim.Height = fmt.Sprint(latest.Height)
			}
		}
		if current.Dimensions.Length == "" {
			newDim.Length = fmt.Sprint(latest.Length)
		} else {
			var length float64
			if _, err := fmt.Sscan(current.Dimensions.Length, &length); err != nil {
				newDim.Length = fmt.Sprint(latest.Length)
			} else if math.Abs(length-latest.Length) > 0.001 {
				newDim.Length = fmt.Sprint(latest.Length)
			}
		}

		if !reflect.ValueOf(newDim).IsZero() {
			ret.Dimensions = &newDim
		}
	}

	if current.Weight == "" {
		ret.Weight = fmt.Sprint(latest.Weight)
	} else {
		var weight float64
		if _, err := fmt.Sscan(current.Weight, &weight); err != nil || math.Abs(weight-latest.Weight) > 0.001 {
			ret.Weight = fmt.Sprint(latest.Weight)
		}
	}

	return &ret, nil
}
