package conjur

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/cyberark/conjur-api-go/conjurapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeTelemetry(t *testing.T, encoded string) string {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(encoded)
	require.NoError(t, err, "telemetry header must be valid base64url")
	return string(b)
}

func TestTelemetryData_FieldsAreSet(t *testing.T) {
	assert.Equal(t, "Secrets Provider For-k8s", telemetryData.IntegrationName)
	assert.Equal(t, "idira-secretsmanager", telemetryData.IntegrationType)
	assert.Equal(t, "Idira", telemetryData.VendorName)
}

func TestTelemetryData_VersionMatchesIntegrationVersion(t *testing.T) {
	assert.Equal(t, IntegrationVersion, telemetryData.IntegrationVersion,
		"telemetryData version must match the IntegrationVersion ldflag variable")
}

func TestTelemetryHeader_ContainsExpectedFields(t *testing.T) {
	// Build the header the same way conjur-api-go does when telemetryData is applied.
	cfg := conjurapi.Config{
		Account:      "myorg",
		ApplianceURL: "http://conjur.example.com",
	}
	cfg.IntegrationName = telemetryData.IntegrationName
	cfg.IntegrationType = telemetryData.IntegrationType
	cfg.IntegrationVersion = telemetryData.IntegrationVersion
	cfg.VendorName = telemetryData.VendorName

	header := cfg.SetFinalTelemetryHeader()
	require.NotEmpty(t, header, "telemetry header must not be empty")

	decoded := decodeTelemetry(t, header)
	assert.True(t, strings.Contains(decoded, "in=Secrets Provider For-k8s"), "decoded=%q", decoded)
	assert.True(t, strings.Contains(decoded, "it=idira-secretsmanager"), "decoded=%q", decoded)
	assert.True(t, strings.Contains(decoded, "vn=Idira"), "decoded=%q", decoded)
}
