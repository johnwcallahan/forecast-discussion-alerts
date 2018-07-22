package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/sfreiberg/gotwilio"
)

// Users struct contains all users
type Users struct {
	Users []User `json:"users"`
}

// User struct represents a user
type User struct {
	ID            int      `json:"id"`
	FirstName     string   `json:"firstName"`
	LastName      string   `json:"lastName"`
	LocationID    string   `json:"locationId"`
	Phone         string   `json:"phone"`
	Subscriptions []string `json:"subscriptions"`
}

// Config struct holds our config
type Config struct {
	TwillioAccountSID string `json:"twillioAccountSID"`
	TwillioAuthToken  string `json:"twillioAuthToken"`
	TwillioFromPhone  string `json:"twillioFromPhone"`
}

// GetSubscribedSections gets all sections of AFD that a user is subscribed to
func (s User) GetSubscribedSections() []string {
	client := NewNWSClient(s.LocationID)
	afd, err := client.GetAFD()
	if err != nil {
		log.Fatal(err)
	}

	sections := make([]string, len(s.Subscriptions))
	for _, subscription := range s.Subscriptions {
		section, err := afd.GetDiscussionSection(subscription)
		if err != nil {
			fmt.Println("Missing section")
		}
		sections = append(sections, section)
	}

	return sections
}

func main() {
	var users Users
	usersFile, err := os.Open("users.json")
	defer usersFile.Close()
	if err != nil {
		log.Fatal(err.Error())
	}
	jsonParser := json.NewDecoder(usersFile)
	jsonParser.Decode(&users)

	var config Config
	configFile, err := os.Open("config_dev.json")
	if err != nil {
		log.Fatal(err.Error())
	}
	jsonParser = json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	twilio := gotwilio.NewTwilioClient(config.TwillioAccountSID, config.TwillioAuthToken)

	for _, user := range users.Users {
		discussionSections := user.GetSubscribedSections()
		for _, section := range discussionSections {
			_, _, err := twilio.SendSMS(config.TwillioFromPhone, user.Phone, section, "", "")
			if err != nil {
				fmt.Println("ERROR")
				fmt.Println(err)
			}
		}
	}
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

// GetDiscussionSection gets a section of the forecast discussion
func (s *Product) GetDiscussionSection(sectionName string) (string, error) {
	sectionName = strings.ToLower(sectionName)

	sectionName = strings.ToUpper(sectionName)
	re := regexp.MustCompile(`(?is)\.` + sectionName + `[.\s]+(.+?)&&`)
	result := re.FindStringSubmatch(s.ProductText)

	if len(result) < 2 {
		return "", errors.New("No section of type " + sectionName + " found")
	}

	section := sanitizeString(result[1])
	section = formatDiscussionItem(sectionName, section)
	return section, nil
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

// -----------------------------------------------------------------------------
// HELPERS
// -----------------------------------------------------------------------------
func sanitizeString(s string) string {
	leadingTrailingWhitespaceRe := regexp.MustCompile(`^[\s\p{Zs}]+|[\s\p{Zs}]+$`)
	multipleNewlineRe := regexp.MustCompile(`([^\n])(\n)([^\n])`)
	multipleSpacesRe := regexp.MustCompile(`(?m) {2,}`)
	tabRe := regexp.MustCompile(`[\t\r]`)

	output := leadingTrailingWhitespaceRe.ReplaceAllString(s, "")
	output = multipleNewlineRe.ReplaceAllString(output, "$1$3")
	output = multipleSpacesRe.ReplaceAllString(output, "")
	output = tabRe.ReplaceAllString(output, "")
	return output
}

func formatDiscussionItem(discussionType string, discussionItem string) string {
	return strings.ToUpper(discussionType) + ":\n\n" + discussionItem
}
