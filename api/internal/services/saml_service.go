package services

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"

	"github.com/docshare/api/internal/config"
	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/pkg/logger"
)

type SAMLService struct {
	Cfg *config.Config
}

func NewSAMLService(cfg *config.Config) *SAMLService {
	return &SAMLService{Cfg: cfg}
}

type SAMLMetadata struct {
	XMLName         xml.Name        `xml:"md:EntityDescriptor"`
	EntityID        string          `xml:"entityID,attr"`
	SPSSODescriptor SPSSODescriptor `xml:"md:SPSSODescriptor"`
}

type SPSSODescriptor struct {
	ProtocolSupportEnumeration string                    `xml:"protocolSupportEnumeration,attr"`
	AttributeConsumingService  AttributeConsumingService `xml:"md:AttributeConsumingService"`
}

type AttributeConsumingService struct {
	ServiceIndex string `xml:"serviceIndex,attr"`
}

func (s *SAMLService) GetMetadata() []byte {
	if !s.Cfg.SAML.Enabled {
		return nil
	}

	metadata := SAMLMetadata{
		EntityID: s.Cfg.SAML.SPEntityID,
		SPSSODescriptor: SPSSODescriptor{
			ProtocolSupportEnumeration: "urn:oasis:names:tc:SAML:2.0:protocol",
			AttributeConsumingService: AttributeConsumingService{
				ServiceIndex: "0",
			},
		},
	}

	bytes, err := xml.MarshalIndent(metadata, "", "  ")
	if err != nil {
		logger.Error("saml_metadata_marshal_failed", errors.New(err.Error()), map[string]interface{}{})
		return nil
	}

	return bytes
}

func (s *SAMLService) HandleACS(ctx context.Context, responseXML string) (*SSOProfile, error) {
	if !s.Cfg.SAML.Enabled {
		return nil, errors.New("SAML is not enabled")
	}

	decoded, err := base64.StdEncoding.DecodeString(responseXML)
	if err != nil {
		return nil, fmt.Errorf("failed to decode SAML response: %w", err)
	}

	var response struct {
		Assertion Assertion `xml:"Assertion"`
	}

	if err := xml.Unmarshal(decoded, &response); err != nil {
		return nil, fmt.Errorf("failed to parse SAML response: %w", err)
	}

	assertion := response.Assertion
	if assertion.Subject.NameID.Value == "" {
		return nil, errors.New("SAML response missing NameID")
	}

	email := ""
	firstName := ""
	lastName := ""

	for _, attr := range assertion.Attribute {
		if attr.Name == "email" || attr.FriendlyName == "email" {
			if len(attr.AttributeValue) > 0 {
				email = attr.AttributeValue[0].Value
			}
		}
		if attr.Name == "firstName" || attr.FriendlyName == "firstName" {
			if len(attr.AttributeValue) > 0 {
				firstName = attr.AttributeValue[0].Value
			}
		}
		if attr.Name == "lastName" || attr.FriendlyName == "lastName" {
			if len(attr.AttributeValue) > 0 {
				lastName = attr.AttributeValue[0].Value
			}
		}
	}

	if email == "" {
		email = assertion.Subject.NameID.Value
	}

	return &SSOProfile{
		Provider:       models.SSOProviderTypeSAML,
		ProviderUserID: assertion.Subject.NameID.Value,
		Email:          email,
		FirstName:      firstName,
		LastName:       lastName,
		RawProfile: map[string]interface{}{
			"name_id":    assertion.Subject.NameID.Value,
			"email":      email,
			"first_name": firstName,
			"last_name":  lastName,
		},
	}, nil
}

type Assertion struct {
	Subject   Subject     `xml:"Subject"`
	Attribute []Attribute `xml:"Attribute"`
}

type Subject struct {
	NameID NameID `xml:"NameID"`
}

type NameID struct {
	Value string `xml:",chardata"`
}

type Attribute struct {
	Name           string           `xml:"Name,attr"`
	FriendlyName   string           `xml:"FriendlyName,attr"`
	AttributeValue []AttributeValue `xml:"AttributeValue"`
}

type AttributeValue struct {
	Value string `xml:",chardata"`
	Type  string `xml:"type,attr"`
}

func (s *SAMLService) IsEnabled() bool {
	return s.Cfg != nil && s.Cfg.SAML.Enabled
}

func (s *SAMLService) ServeMetadata(w http.ResponseWriter, r *http.Request) {
	metadata := s.GetMetadata()
	if metadata == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	if _, err := w.Write(metadata); err != nil {
		logger.Error("saml_metadata_write_failed", err, nil)
	}
}
