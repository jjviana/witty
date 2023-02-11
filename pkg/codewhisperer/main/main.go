package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssooidc"
	"github.com/jjviana/codex/pkg/codewhisperer/service"
	"net/http"
	"os"
	"time"
)

// Command: completion
// args: "prefix"
// Calls CodeWhisperer recommendation completion api and prints the result.

type BearerHTTPRoundTRipper struct {
	http.RoundTripper
	Token string
}

func (r *BearerHTTPRoundTRipper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", "Bearer "+r.Token)
	return r.RoundTripper.RoundTrip(req)
}

func main() {

	httpBearer := &BearerHTTPRoundTRipper{RoundTripper: http.DefaultTransport}
	// Create a custom HTTP client that will send the Bearer token
	// in the Authorization header.
	httpClient := &http.Client{
		Transport: httpBearer,
	}

	// Create a CodeWhisperer client from just a session.
	sess := session.Must(session.NewSession())
	//svc := codewhisperer.New(sess)

	// Create a CodeWhisperer client with additional configuration
	svc := service.New(sess, aws.NewConfig().WithRegion("us-east-1").WithCredentials(
		credentials.AnonymousCredentials).WithEndpoint("https://codewhisperer.us-east-1.amazonaws.com").
		WithHTTPClient(httpClient))

	// Create a ssooidc unmanaged client
	ssoidc := ssooidc.New(sess, aws.NewConfig().WithRegion("us-east-1").WithCredentials(
		credentials.AnonymousCredentials))

	bearerToken, err := authenticate(ssoidc)
	if err != nil {
		fmt.Printf("Error: %s", err)
		return
	}

	// Set the Bearer token in the HTTP client.
	httpBearer.Token = *bearerToken.AccessToken

	fileName := "dummy.py"
	languageName := "python"
	rightFileContent := ""
	// Call the CodeWhisperer recommendation completion api.
	result, err := svc.GenerateCompletions(&service.GenerateCompletionsInput{
		FileContext: &service.FileContext{
			Filename:         &fileName,
			LeftFileContent:  &os.Args[1],
			RightFileContent: &rightFileContent,
			ProgrammingLanguage: &service.ProgrammingLanguage{
				LanguageName: &languageName,
			},
		},
	})

	if err != nil {
		fmt.Printf("Error: %s", err)
		return
	}

	fmt.Printf("Got %d completions\n", len(result.Completions))
	for _, completion := range result.Completions {
		fmt.Printf("Completion: %s\n", *completion.Content)
	}
}

func authenticate(sso *ssooidc.SSOOIDC) (*ssooidc.CreateTokenOutput, error) {

	token := loadCachedToken()
	if token != nil {
		return token, nil
	}
	clientRegistration, err := registerClient(sso)
	if err != nil {
		return nil, err
	}

	authStart, err := authorizeClient(sso, clientRegistration)
	if err != nil {
		return nil, err
	}

	// Poll for authentication
	pollInterval := time.Duration(*authStart.Interval) * time.Second
	for {
		authResult, err := sso.CreateToken(&ssooidc.CreateTokenInput{
			ClientId:     clientRegistration.ClientId,
			ClientSecret: clientRegistration.ClientSecret,
			GrantType:    aws.String("urn:ietf:params:oauth:grant-type:device_code"),
			DeviceCode:   authStart.DeviceCode,
		})
		switch {
		case err != nil && err.(awserr.Error).Code() == ssooidc.ErrCodeAuthorizationPendingException:
			// Continue polling
		case err != nil && err.(awserr.Error).Code() == ssooidc.ErrCodeSlowDownException:
			// Slow down
			pollInterval = pollInterval * 2
		default:
			if err != nil {
				return nil, err
			}
		}

		if authResult.AccessToken != nil {
			saveToken(authResult)
			return authResult, nil
		}
		time.Sleep(pollInterval)
	}

}

func authorizeClient(sso *ssooidc.SSOOIDC, clientRegistration *ssooidc.RegisterClientOutput) (*ssooidc.StartDeviceAuthorizationOutput, error) {

	out, err := sso.StartDeviceAuthorization(&ssooidc.StartDeviceAuthorizationInput{
		ClientId:     clientRegistration.ClientId,
		ClientSecret: clientRegistration.ClientSecret,
		StartUrl:     aws.String("https://view.awsapps.com/start"),
	})
	if err != nil {
		return nil, err
	}

	fmt.Printf("Visit %s and enter code %s\n", *out.VerificationUriComplete, *out.UserCode)
	return out, nil

}
func registerClient(sso *ssooidc.SSOOIDC) (*ssooidc.RegisterClientOutput, error) {

	registration := loadCachedRegistration()
	if registration != nil {
		return registration, nil
	}

	clientRegistration, err := sso.RegisterClient(&ssooidc.RegisterClientInput{
		ClientType: aws.String("public"),
		ClientName: aws.String(fmt.Sprintf("witty-%d", time.Now().Unix())),
		Scopes: []*string{aws.String("codewhisperer:completions"),
			aws.String("codewhisperer:analysis")},
	})
	if err != nil {
		return nil, err
	}
	cacheRegistration(clientRegistration)
	return clientRegistration, nil
}

func loadCachedValue(fileName string, value interface{}) error {

	homeDir := os.Getenv("HOME")
	f, err := os.Open(homeDir + "/.witty/" + fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	err = decoder.Decode(value)
	if err != nil {
		return err
	}
	return nil

}

func saveValue(fileName string, value interface{}) error {

	homeDir := os.Getenv("HOME")
	dir, err := os.Stat(homeDir + "/.witty")
	if err != nil {
		err = os.Mkdir(homeDir+"/.witty", 0700)
		if err != nil {
			return err
		}
	} else {
		if !dir.IsDir() {
			return errors.New("Cannot create directory " + homeDir + "/.witty")
		}
	}

	f, err := os.Create(homeDir + "/.witty/" + fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	err = encoder.Encode(value)
	if err != nil {
		return err
	}
	return nil

}

func loadCachedRegistration() *ssooidc.RegisterClientOutput {
	// Read registration stored previously as json in file
	// ~/.witty/registration.json

	var registration ssooidc.RegisterClientOutput
	err := loadCachedValue("registration.json", &registration)
	if err != nil {
		fmt.Printf("Error loading cached registration: %s", err)
		return nil
	}
	fmt.Printf("Loaded client registration: %s\n", *registration.ClientId)
	return &registration

}

func cacheRegistration(registration *ssooidc.RegisterClientOutput) {
	// Write registration to file ~/.witty/registration.json

	err := saveValue("registration.json", registration)
	if err != nil {
		fmt.Printf("Error saving registration: %s", err)
	}

}

func loadCachedToken() *ssooidc.CreateTokenOutput {
	// Read token stored previously as json in file
	// ~/.witty/token.json

	var token ssooidc.CreateTokenOutput
	err := loadCachedValue("token.json", &token)
	if err != nil {
		fmt.Printf("Error loading cached token: %s", err)
		return nil
	}
	fmt.Printf("Loaded token: %s\n", *token.AccessToken)
	return &token

}

func saveToken(token *ssooidc.CreateTokenOutput) {
	// Write token to file ~/.witty/token.json

	err := saveValue("token.json", token)
	if err != nil {
		fmt.Printf("Error saving token: %s", err)
	}

}
