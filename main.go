package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func main() {
	client := NewNWSClient("PHI")
	afd, err := client.GetAFD()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(afd.ProductText)
}

// Response struct which contains multiple products
type Response struct {
	Products []Product `json:"@graph"`
}

// Product struct which represents a product listing
type Product struct {
	ID              string `json:"id"`
	WmoCollectiveID string `json:"wmoCollectiveId"`
	IssuingOffice   string `json:"issuingOffice"`
	IssuanceTime    string `json:"issuanceTime"`
	ProductCode     string `json:"productCode"`
	ProductName     string `json:"productName"`
	ProductText     string `json:"productText"`
}

// NWSClient struct is a wrapper around the NWS API
type NWSClient struct {
	LocationID string
	BaseURI    string
}

// NewNWSClient returns a client with default params
func NewNWSClient(locationID string) *NWSClient {
	return &NWSClient{
		LocationID: locationID,
		BaseURI:    "https://api.weather.gov",
	}
}

func (s *NWSClient) doRequest(req *http.Request) ([]byte, error) {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s", body)
	}
	return body, nil
}

// GetAFD return most recent Area Forecast Discussion
func (s *NWSClient) GetAFD() (*Product, error) {
	products, err := s.GetProducts("afd")
	if err != nil {
		return nil, err
	}

	// TODO: Error handle if there are no products
	if len(products) < 1 {
		return nil, errors.New("Couldn't find AFD")
	}

	latestID := products[0].ID
	afd, err := s.GetProduct(latestID)
	if err != nil {
		return nil, err
	}
	return afd, nil
}

// GetProducts methods retrieves product listing
func (s *NWSClient) GetProducts(productType string) ([]Product, error) {
	uri := s.BaseURI + "/products/types/" + productType + "/locations/" + s.LocationID
	req, err := http.NewRequest("GET", uri, nil)
	req.Header.Set("Accept", "application/geo+json")
	if err != nil {
		return nil, err
	}
	bytes, err := s.doRequest(req)
	if err != nil {
		return nil, err
	}
	var resp Response
	err = json.Unmarshal(bytes, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Products, nil
}

// GetProduct returns a single product from the API by ID
func (s *NWSClient) GetProduct(productID string) (*Product, error) {
	uri := s.BaseURI + "/products/" + productID
	req, err := http.NewRequest("GET", uri, nil)
	req.Header.Set("Accept", "application/geo+json")
	if err != nil {
		return nil, err
	}
	bytes, err := s.doRequest(req)
	if err != nil {
		return nil, err
	}
	var product Product
	err = json.Unmarshal(bytes, &product)
	if err != nil {
		return nil, err
	}
	return &product, nil
}