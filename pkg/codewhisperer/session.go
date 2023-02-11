package codewhisperer

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssooidc"
	"github.com/jjviana/codex/pkg/codewhisperer/service"
	"github.com/rs/zerolog/log"
	"net/http"
	"time"
)

type configRepository interface {
	Store(name string, config interface{}) error
	Load(name string, config interface{}) error
}

type display interface {
	ShowMessage(message string)
}

const awsRegion = "us-east-1"
const endpoint = "https://codewhisperer.us-east-1.amazonaws.com"
const clientType = "public"
const deviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"
const refreshGrantType = "refresh_token"

var startUrl = aws.String("https://view.awsapps.com/start")

var scopes = []*string{aws.String("codewhisperer:completions"),
	aws.String("codewhisperer:analysis")}

type SessionManager struct {
	configRepository configRepository
	display          display
	bearer           *BearerHTTPRoundTRipper
	httpClient       *http.Client
	service          *service.CodeWhisperer
	ssooidc          *ssooidc.SSOOIDC
	currentToken     *ssooidc.CreateTokenOutput
	client           *ssooidc.RegisterClientOutput
}

func NewSessionManager(configRepository configRepository, display display) *SessionManager {
	bearer := BearerHTTPRoundTRipper{RoundTripper: http.DefaultTransport}
	httpClient := &http.Client{Transport: &bearer}
	awsSession := session.Must(session.NewSession())
	service := service.New(awsSession, aws.NewConfig().WithRegion(awsRegion).WithCredentials(
		credentials.AnonymousCredentials).WithEndpoint(endpoint).
		WithHTTPClient(httpClient))
	ssoidc := ssooidc.New(awsSession, aws.NewConfig().WithRegion(awsRegion).WithCredentials(
		credentials.AnonymousCredentials))

	return &SessionManager{
		configRepository: configRepository,
		display:          display,
		bearer:           &bearer,
		httpClient:       httpClient,
		service:          service,
		ssooidc:          ssoidc,
	}
}

func (s *SessionManager) Start() error {
	client, err := s.loadOrRegisterClient()
	if err != nil {
		return err
	}
	s.client = client
	log.Debug().Msgf("CodeWhisperer Client: %v", client.ClientId)

	token, err := s.loadOrCreateToken()
	if err != nil {
		return err
	}
	s.currentToken = token
	s.bearer.Token = *token.AccessToken
	return nil
}

func (s *SessionManager) loadOrCreateToken() (*ssooidc.CreateTokenOutput, error) {
	token := &ssooidc.CreateTokenOutput{}
	err := s.configRepository.Load("codewhisperer-token", token)
	if err != nil {
		token, err = s.authorizeClient()
		if err != nil {
			return nil, err
		}
		err = s.configRepository.Store("codewhisperer-token", token)
		if err != nil {
			return nil, err
		}
	}
	return token, nil
}

func (s *SessionManager) authorizeClient() (*ssooidc.CreateTokenOutput, error) {

	authorization, err := s.ssooidc.StartDeviceAuthorization(&ssooidc.StartDeviceAuthorizationInput{
		ClientId:     s.client.ClientId,
		ClientSecret: s.client.ClientSecret,
		StartUrl:     startUrl,
	})
	if err != nil {
		return nil, err
	}
	s.display.ShowMessage(fmt.Sprintf("Please visit the following URL to authorize this application to use CodeWhisperer:\n %s\n",
		*authorization.VerificationUriComplete))

	return s.pollForToken(authorization)

}

func (s *SessionManager) pollForToken(authorization *ssooidc.StartDeviceAuthorizationOutput) (*ssooidc.CreateTokenOutput, error) {
	// Poll for authentication
	pollInterval := time.Duration(*authorization.Interval) * time.Second
	for {

		authResult, err := s.ssooidc.CreateToken(&ssooidc.CreateTokenInput{
			ClientId:     s.client.ClientId,
			ClientSecret: s.client.ClientSecret,
			GrantType:    aws.String(deviceGrantType),
			DeviceCode:   authorization.DeviceCode,
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
			return authResult, nil
		}
		time.Sleep(pollInterval)
	}
}

func (s *SessionManager) GenerateCompletions(request *service.GenerateCompletionsInput) (*service.GenerateCompletionsOutput, error) {
	response, err := s.service.GenerateCompletions(request)
	if err != nil {
		log.Debug().Msgf("Error calling GenerateCompletions: %v", err)
		if err.(awserr.Error).Code() == ssooidc.ErrCodeExpiredTokenException ||
			err.(awserr.Error).Code() == ssooidc.ErrCodeAccessDeniedException {
			log.Debug().Msgf("Refreshing token")
			// Refresh token
			token, err := s.refreshToken()
			if err != nil {
				log.Debug().Msgf("Error refreshing token: %v", err)
				return nil, err
			}
			s.currentToken = token
			s.bearer.Token = *token.AccessToken
			return s.service.GenerateCompletions(request)
		}
	}
	return response, nil
}

func (s *SessionManager) refreshToken() (*ssooidc.CreateTokenOutput, error) {
	token, err := s.ssooidc.CreateToken(&ssooidc.CreateTokenInput{
		ClientId:     s.client.ClientId,
		ClientSecret: s.client.ClientSecret,
		GrantType:    aws.String(refreshGrantType),
		RefreshToken: s.currentToken.RefreshToken,
	})
	if err != nil {
		return nil, err
	}
	err = s.configRepository.Store("codewhisperer-token", token)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func (s *SessionManager) loadOrRegisterClient() (*ssooidc.RegisterClientOutput, error) {
	client := &ssooidc.RegisterClientOutput{}
	err := s.configRepository.Load("codewhisperer-client", client)
	if err != nil {
		client, err = s.registerClient()
		if err != nil {
			return nil, err
		}
		err = s.configRepository.Store("codewhisperer-client", client)
		if err != nil {
			return nil, err
		}
	}
	return client, nil
}

func (s *SessionManager) registerClient() (*ssooidc.RegisterClientOutput, error) {

	clientRegistration, err := s.ssooidc.RegisterClient(&ssooidc.RegisterClientInput{
		ClientType: aws.String(clientType),
		ClientName: aws.String(fmt.Sprintf("witty-%d", time.Now().Unix())),
		Scopes:     scopes,
	})
	if err != nil {
		return nil, err
	}
	return clientRegistration, nil
}
