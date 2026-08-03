package petstore

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/suite"
	"github.com/go-resty/resty/v2"
)

type PetStoreSuite struct {
	suite.Suite
	transport *httpmock.MockTransport
	client    ClientInterface
}

const Prefix = "http://invalid.localdomain/prefix"

func (suite *PetStoreSuite) SetupTest() {
	var err error
	suite.transport = httpmock.NewMockTransport()
	suite.client, err = NewClient(Prefix, WithRoundTripper(suite.transport))

	suite.Require().Nil(err)
}

func TestPetStoreSuite(t *testing.T) {
	suite.Run(t, new(PetStoreSuite))
}

func (suite *PetStoreSuite) TestInterfaces() {
	// Since the client code is generated, we make "copies" of this code to ensure that the client we generate
	// maintains the inteface we desire

	type DesiredPetClient interface {
		Add(ctx context.Context, body *Pet) (*Pet, error)
		AddResponse(ctx context.Context, body *Pet) (*resty.Response, error)

		Delete(ctx context.Context, petId int64, params *DeletePetParams) error
		DeleteResponse(ctx context.Context, petId int64, params *DeletePetParams) (*resty.Response, error)
	}
	type DesiredUserClient interface {
		Create(ctx context.Context, body *User) (*User, error)
		CreateResponse(ctx context.Context, body *User) (*resty.Response, error)
	}

	type DesiredClientInterface interface {
		Pet() PetClient
		User() UserClient
	}

	_, ok := suite.client.(DesiredClientInterface)

	suite.Require().True(ok, "ClientInterface implements DesiredClientInterface")

	_, ok = suite.client.Pet().(DesiredPetClient)
	suite.True(ok, "PetClient implements DesiredPetClient")

	_, ok = suite.client.User().(DesiredUserClient)
	suite.True(ok, "UserClient implements DesiredUserClient")
}

func (suite *PetStoreSuite) TestAddOk() {
	suite.transport.RegisterResponder("POST", Prefix+"/pet",
		httpmock.NewJsonResponderOrPanic(200, map[string]any{
			"id":        123,
			"name":      "Midnight",
			"photoUrls": nil,
			"status":    "sold",
		}),
	)

	var id int64 = 123
	status := PetStatus("sold")
	resp, err := suite.client.Pet().Add(context.Background(), &Pet{
		Category: &Category{},
		Id:       &id,
		Name:     "Midnight",
		Status:   &status,
	})
	suite.NoError(err)
	suite.NotNil(resp, "%s", resp)
	suite.EqualValues(*resp.Id, 123)
	suite.Equal(resp.Name, "Midnight")
}

func (suite *PetStoreSuite) TestAddResponseErrorHTML() {
	suite.transport.RegisterResponder(
		"POST",
		Prefix+"/pet",
		httpmock.NewStringResponder(500, "<html><body><h1>Internal Server Error"),
	)

	resp, err := suite.client.Pet().AddResponse(context.Background(), &Pet{})
	suite.NotNil(resp)
	suite.ErrorContains(err, "server error '500 Internal Server Error'")
	target := &GeneralHTTPError{}
	suite.Require().ErrorAs(err, &target)

	suite.Equal("<html><body><h1>Internal Server Error", target.Text)
	suite.Nil(target.JSON)
}

func (suite *PetStoreSuite) TestAddResponseErrorJSON() {
	suite.transport.RegisterResponder("POST", Prefix+"/pet",
		httpmock.NewJsonResponderOrPanic(404, map[string]any{
			"error": "uh-oh",
		}),
	)
	resp, err := suite.client.Pet().AddResponse(context.Background(), &Pet{})
	suite.NotNil(resp)

	suite.ErrorContains(err, "client error '404 Not Found'")
	target := &GeneralHTTPError{}
	suite.Require().ErrorAs(err, &target)
	suite.Equal("", target.Text)
	suite.Equal(map[string]any{"error": "uh-oh"}, target.JSON)
}

func (suite *PetStoreSuite) TestNetworkErrors() {
	suite.transport.RegisterResponder("POST", Prefix+"/pet",
		func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("some network timeout")
		},
	)
	resp, err := suite.client.Pet().AddResponse(context.Background(), &Pet{})

	suite.Equal(1, suite.transport.GetTotalCallCount())

	suite.NotNil(resp, "Resp object is returned even in case of network error")
	target := &url.Error{}
	suite.Require().ErrorAs(err, &target)
	suite.EqualError(err, `Post "http://invalid.localdomain/prefix/pet": some network timeout`)
}

func (suite *PetStoreSuite) TestDelete() {
	apiKey := "abc"
	params := DeletePetParams{ApiKey: &apiKey}

	matcher := httpmock.NewMatcher("", func(req *http.Request) bool {
		return req.Header.Get("api_key") == apiKey
	})
	suite.transport.RegisterMatcherResponder("DELETE", Prefix+"/pet/12", matcher,
		httpmock.NewBytesResponder(200, nil),
	)
	err := suite.client.Pet().Delete(context.Background(), 12, &params)
	suite.NoError(err)
}

func (suite *PetStoreSuite) TestFindByTags() {
	// Test that optional params sends the correct requests
	tags := []string{"abc", "def"}

	cases := map[string]struct {
		params *FindPetsByTagsParams
		mf     httpmock.MatcherFunc
	}{
		"nil-params": {
			params: nil,
			mf: func(req *http.Request) bool {
				suite.Equal(req.URL.RawQuery, "")
				return true
			},
		},
		"empty-params": {
			params: &FindPetsByTagsParams{},
			mf: func(req *http.Request) bool {
				suite.Equal(req.URL.RawQuery, "")
				return true
			},
		},
		"with-tags": {
			params: &FindPetsByTagsParams{Tags: &tags},
			mf: func(req *http.Request) bool {
				suite.Equal(req.URL.RawQuery, "tags=abc&tags=def")
				return true
			},
		},
	}

	for name, tc := range cases {
		suite.Run(name, func() {
			matcher := httpmock.NewMatcher("", tc.mf)
			suite.transport.Reset()
			suite.transport.RegisterMatcherResponder("GET", Prefix+"/pet/findByTags", matcher,
				httpmock.NewBytesResponder(200, nil),
			)

			_, err := suite.client.Pet().FindByTags(context.Background(), tc.params)
			suite.NoError(err)
		})
	}
}

func (suite *PetStoreSuite) TestFindByStatus() {
	suite.transport.RegisterResponder("GET", Prefix+"/pet/findByStatus",
		httpmock.NewBytesResponder(200, nil),
	)

	_, err := suite.client.Pet().FindByStatus(context.Background(), nil)
	suite.EqualError(err, "FindByStatus requires a non-nil params argument")

	_, err = suite.client.Pet().FindByStatus(context.Background(), &FindPetsByStatusParams{})
	suite.NoError(err)
}

func (suite *PetStoreSuite) TestCreateUser() {
	body := &User{}

	suite.transport.RegisterResponder("POST", Prefix+"/user",
		httpmock.NewJsonResponderOrPanic(200, body),
	)

	_, err := suite.client.User().Create(context.Background(), body)
	suite.NoError(err)
}
