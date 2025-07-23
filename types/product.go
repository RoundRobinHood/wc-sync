package types

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrProductNotFound = errors.New("Product not found")

type PriceString string

func (p *PriceString) UnmarshalJSON(data []byte) error {
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}

	*p = PriceString(data)
	return nil
}

func (p PriceString) MarshalJSON() ([]byte, error) {
	var f float64
	if err := json.Unmarshal([]byte(p), &f); err != nil {
		return nil, err
	}

	return []byte(p), nil
}

type ZonelessTimestamp struct {
	Time *time.Time
}

const customLayout = "2006-01-02T15:04:05.99"

func (z *ZonelessTimestamp) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" {
		z.Time = nil
		return nil
	}
	t, err := time.Parse(customLayout, s)
	if err != nil {
		return err
	}

	z.Time = &t
	return nil
}

func (z ZonelessTimestamp) MarshalJSON() ([]byte, error) {
	if z.Time == nil {
		return []byte("null"), nil
	}
	return []byte(`"` + z.Time.Format(customLayout) + `"`), nil
}

type YN bool

func (y *YN) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	*y = strings.ToLower(s) == "yes"
	return nil
}

func (y YN) MarshalJSON() ([]byte, error) {
	if y {
		return []byte(`"yes"`), nil
	} else {
		return []byte(`"no"`), nil
	}
}

type TarsusProduct struct {
	ProductNumber  string            `json:"Product_Number"`
	PartNr         string            `json:"Manufacturing_Part_Number"`
	ShortDesc      string            `json:"Short_Advertising_Description"`
	Description    string            `json:"Product_Description"`
	BarCode        string            `json:"BarCode"`
	ProductType    string            `json:"Product_Type"`
	Manufacturer   string            `json:"Manufacturer"`
	Category       string            `json:"Category"`
	Stock          int               `json:"Available_Stock"`
	PriceExVAT     PriceString       `json:"Price_ex_Vat"`
	DateAdded      ZonelessTimestamp `json:"Date_Added"`
	ImageURL       string            `json:"Image_URL"`
	ExportDate     ZonelessTimestamp `json:"Export_Date"`
	Serialized     string            `json:"Serialized"`
	ETADate        ZonelessTimestamp `json:"ETA_Date"`
	RealPriceExVat PriceString       `json:"Non_Discount_Price_ex_Vat"`
	DiscountQtty   int               `json:"Discount_Quantity"`
	Discounted     YN                `json:"Product_Discounted"`
	Width          float64           `json:"Each_Width"`
	Height         float64           `json:"Each_Height"`
	Length         float64           `json:"Each_Length"`
	Weight         float64           `json:"Each_Weight"`
}

type WCTag struct {
	Id   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Slug string `json:"slug,omitempty"`
}

type WCCategory struct {
	Id   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Slug string `json:"slug,omitempty"`
}

type WCImage struct {
	Id   int    `json:"id,omitempty"`
	Href string `json:"src,omitempty"`
	Name string `json:"name,omitempty"`
}

type WCDimensions struct {
	Length string `json:"length"`
	Width  string `json:"width"`
	Height string `json:"height"`
}

type WooCommerceProduct struct {
	ID           int           `json:"id,omitempty"`
	SKU          string        `json:"sku,omitempty"`
	Name         string        `json:"name,omitempty"`
	Slug         string        `json:"slug,omitempty"`
	Description  string        `json:"description,omitempty"`
	Tags         []WCTag       `json:"tags,omitempty"`
	ProductType  string        `json:"type,omitempty"`
	Categories   []WCCategory  `json:"categories,omitempty"`
	StockQtty    int           `json:"stock_quantity,omitempty"`
	RegularPrice string        `json:"regular_price,omitempty"`
	Images       []WCImage     `json:"images,omitempty"`
	Dimensions   *WCDimensions `json:"dimensions,omitempty"`
	Weight       string        `json:"weight,omitempty"`
}

type WCProductResponse struct {
	WooCommerceProduct
	Error any `json:"error"`
}
