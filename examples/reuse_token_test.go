//go:build examples

// These examples call Intuit with placeholder credentials; fill them in and
// run with: go test -tags examples ./examples
package examples

import (
	"fmt"
	"testing"

	quickbooks "github.com/scriplylabs/goclient-qbo"
	"github.com/stretchr/testify/require"
)

func TestReuseToken(t *testing.T) {
	clientID := "<your-client-id>"
	clientSecret := "<your-client-secret>"
	realm := "<realm-id>"

	token := quickbooks.BearerToken{
		RefreshToken: "<saved-refresh-token>",
		AccessToken:  "<saved-access-token>",
	}

	qbClient, err := quickbooks.NewClient(clientID, clientSecret, realm, false, "", &token)
	require.NoError(t, err)

	// Make a request!
	info, err := qbClient.FindCompanyInfo()
	require.NoError(t, err)
	fmt.Println(info)
}
