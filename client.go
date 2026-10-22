// Package quickbooks is all the RESTful endpoints
package quickbooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is your handle to the QuickBooks API.
type Client struct {
	// Get this from oauth2.NewClient().
	Client *http.Client
	// Set to ProductionEndpoint or SandboxEndpoint.
	endpoint *url.URL
	// The set of quickbooks APIs
	discoveryAPI *DiscoveryAPI
	// The client Id
	clientID string
	// The client Secret
	clientSecret string
	// The minor version of the QB API
	minorVersion string
	// The account Id you're connecting to.
	realm string
	// Flag set if the limit of 500req/s has been hit (source: https://developer.intuit.com/app/developer/qbo/docs/learn/rest-api-features#limits-and-throttles)
	throttled bool
}

// NewClient initializes a new QuickBooks client for interacting with their Online API
func NewClient(clientID string, clientSecret string, realm string, isProduction bool, minorVersion string, token *BearerToken) (c *Client, err error) {
	if minorVersion == "" {
		minorVersion = "65"
	}

	client := Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		minorVersion: minorVersion,
		realm:        realm,
		throttled:    false,
	}

	if isProduction {
		client.endpoint, err = url.Parse(ProductionEndpoint.String() + "/v3/company/" + realm + "/")
		if err != nil {
			return nil, fmt.Errorf("failed to parse API endpoint: %v", err)
		}

		client.discoveryAPI, err = CallDiscoveryAPI(DiscoveryProductionEndpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain discovery endpoint: %v", err)
		}
	} else {
		client.endpoint, err = url.Parse(SandboxEndpoint.String() + "/v3/company/" + realm + "/")
		if err != nil {
			return nil, fmt.Errorf("failed to parse API endpoint: %v", err)
		}

		client.discoveryAPI, err = CallDiscoveryAPI(DiscoverySandboxEndpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain discovery endpoint: %v", err)
		}
	}

	if token != nil {
		client.Client = getHttpClient(token)
	}

	return &client, nil
}

// NewClientWithEndpoint returns a client that sends API calls to endpoint (a
// base URL such as "http://127.0.0.1:8080", without "/v3/company/...")
// instead of ProductionEndpoint or SandboxEndpoint; qbotest.NewServer uses it.
// Unlike NewClient it doesn't fetch Intuit's discovery document, so the OAuth
// methods (FindAuthorizationURL, RetrieveBearerToken, RefreshToken,
// RevokeToken) return an error. A nil token leaves Client unset, for the
// caller to fill in.
func NewClientWithEndpoint(endpoint EndpointURL, realm string, minorVersion string, token *BearerToken) (*Client, error) {
	if minorVersion == "" {
		minorVersion = "65"
	}

	apiEndpoint, err := url.Parse(strings.TrimSuffix(endpoint.String(), "/") + "/v3/company/" + realm + "/")
	if err != nil {
		return nil, fmt.Errorf("failed to parse API endpoint: %v", err)
	}

	client := Client{
		endpoint:     apiEndpoint,
		minorVersion: minorVersion,
		realm:        realm,
	}

	if token != nil {
		client.Client = getHttpClient(token)
	}

	return &client, nil
}

// getDiscoveryAPI returns the client's OAuth endpoints, or an error for a client
// made by NewClientWithEndpoint.
func (c *Client) getDiscoveryAPI() (*DiscoveryAPI, error) {
	if c.discoveryAPI == nil {
		return nil, errors.New("client has no OAuth endpoints; it was made by NewClientWithEndpoint")
	}

	return c.discoveryAPI, nil
}

// FindAuthorizationURL compiles the authorization url from the discovery api's auth endpoint.
//
// Example: qbClient.FindAuthorizationURL("com.intuit.quickbooks.accounting", "security_token", "https://developer.intuit.com/v2/OAuth2Playground/RedirectUrl")
//
// You can find live examples from https://developer.intuit.com/app/developer/playground
func (c *Client) FindAuthorizationURL(scope string, state string, redirectURI string) (string, error) {
	var authorizationURL *url.URL

	discoveryAPI, err := c.getDiscoveryAPI()
	if err != nil {
		return "", err
	}

	authorizationURL, err = url.Parse(discoveryAPI.AuthorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("failed to parse auth endpoint: %v", err)
	}

	urlValues := url.Values{}
	urlValues.Add("client_id", c.clientID)
	urlValues.Add("response_type", "code")
	urlValues.Add("scope", scope)
	urlValues.Add("redirect_uri", redirectURI)
	urlValues.Add("state", state)
	authorizationURL.RawQuery = urlValues.Encode()

	return authorizationURL.String(), nil
}

func (c *Client) req(method string, endpoint string, payloadData any, responseObject any, queryParameters map[string]string) (e error) {
	// TODO: possibly just wait until c.throttled is false, and continue the request?
	if c.throttled {
		return errors.New("waiting for rate limit")
	}

	endpointURL := *c.endpoint
	endpointURL.Path += endpoint
	urlValues := url.Values{}

	if len(queryParameters) > 0 {
		for param, value := range queryParameters {
			urlValues.Add(param, value)
		}
	}

	urlValues.Set("minorversion", c.minorVersion)
	urlValues.Encode()
	endpointURL.RawQuery = urlValues.Encode()

	var err error
	var marshalledJson []byte

	if payloadData != nil {
		marshalledJson, err = json.Marshal(payloadData)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %v", err)
		}
	}

	req, err := http.NewRequest(method, endpointURL.String(), bytes.NewBuffer(marshalledJson))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && e == nil {
			e = closeErr
		}
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		break
	case http.StatusTooManyRequests:
		c.throttled = true
		go func(c *Client) {
			time.Sleep(1 * time.Minute)
			c.throttled = false
		}(c)
	default:
		return parseFailure(resp)
	}

	if responseObject != nil {
		if err = json.NewDecoder(resp.Body).Decode(&responseObject); err != nil {
			return fmt.Errorf("failed to unmarshal response into object: %v", err)
		}
	}

	return nil
}

// RequestOption sets an optional query parameter on a request.
type RequestOption func(queryParameters map[string]string)

// WithRequestID sets QuickBooks' requestid query parameter. When QuickBooks
// sees a requestid it has seen before for the company, it returns the
// original response instead of acting again, even if the body differs, so a
// retried create can't make a second entry. Use one ID per logical operation
// and reuse it on every retry of that operation.
//
// The ID can be at most 50 characters; a longer one is refused with a
// ValidationFault (code 6000). A UUID fits. Intuit doesn't publish how long
// QuickBooks remembers an ID, and says a repeat sent while the first request
// is still being processed isn't guaranteed to match it.
func WithRequestID(id string) RequestOption {
	return func(queryParameters map[string]string) {
		queryParameters["requestid"] = id
	}
}

// buildQueryParameters applies opts to a new set of query parameters.
func buildQueryParameters(opts []RequestOption) map[string]string {
	params := map[string]string{}
	for _, opt := range opts {
		opt(params)
	}

	return params
}

func (c *Client) get(endpoint string, responseObject interface{}, queryParameters map[string]string) error {
	return c.req("GET", endpoint, nil, responseObject, queryParameters)
}

func (c *Client) post(endpoint string, payloadData interface{}, responseObject interface{}, queryParameters map[string]string) error {
	return c.req("POST", endpoint, payloadData, responseObject, queryParameters)
}

// query makes the specified QBO `query` and unmarshals the result into `responseObject`
func (c *Client) query(query string, responseObject interface{}) error {
	return c.get("query", responseObject, map[string]string{"query": query})
}
