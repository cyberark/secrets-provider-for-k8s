//go:build e2e
// +build e2e

package e2e

const (
	// Container names
	TestAppContainer       = "test-app"
	ConjurClusterContainer = "conjur-appliance"
	ConjurCLIContainer     = "conjur-cli"
	LogsContainer          = "cyberark-secrets-provider-for-k8s"

	// Label selectors
	SPLabelSelector             = "app=test-env"
	SPHelmLabelSelector         = "app=test-helm"
	ConjurCLILabelSelector      = "app=conjur-cli"
	ConjurFollowerLabelSelector = "role=follower"

	// Available templates
	K8sTemplate         = "secrets-provider-init-container"
	K8sRotationTemplate = "secrets-provider-k8s-rotation"
	P2fTemplate         = "secrets-provider-init-push-to-file"
	P2fRotationTemplate = "secrets-provider-p2f-rotation"

	FetchAllJSONContent           = `{"secrets/another_test_secret":"some-secret","secrets/encoded":"c2VjcmV0LXZhbHVl","secrets/json_object_secret":"\"{\"auths\":{\"someurl\":{\"auth\":\"sometoken=\"}}}\"","secrets/password":"7H1SiSmYp@5Sw0rd","secrets/ssh_key":"\"ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA879BJGYlPTLIuc9/R5MYiN4yc/YiCLcdBpSdzgK9Dt0Bkfe3rSz5cPm4wmehdE7GkVFXrBJ2YHqPLuM1yx1AUxIebpwlIl9f/aUHOts9eVnVh4NztPy0iSU/Sv0b2ODQQvcy2vYcujlorscl8JjAgfWsO3W4iGEe6QwBpVomcME8IU35v5VbylM9ORQa6wvZMVrPECBvwItTY8cPWH3MGZiK/74eHbSLKA4PY3gM4GHI450Nie16yggEg2aTQfWA1rry9JYWEoHS9pJ1dnLqZU3k/8OWgqJrilwSoC5rGjgp93iu0H8T6+mEHGRQe84Nk1y5lESSWIbn6P636Bl3uQ== your@email.com\"","secrets/test_secret":"supersecret","secrets/umlaut":"ÄäÖöÜü","secrets/url":"postgresql://test-app-backend.app-test.svc.cluster.local:5432","secrets/username":"some-user","secrets/var with spaces":"some-secret","secrets/var+with+pluses":"some-secret"}`
	FetchAllBase64TemplateContent = `secrets/another_test_secret: c29tZS1zZWNyZXQ=
secrets/encoded: YzJWamNtVjBMWFpoYkhWbA==
secrets/json_object_secret: InsiYXV0aHMiOnsic29tZXVybCI6eyJhdXRoIjoic29tZXRva2VuPSJ9fX0i
secrets/password: N0gxU2lTbVlwQDVTdzByZA==
secrets/ssh_key: InNzaC1yc2EgQUFBQUIzTnphQzF5YzJFQUFBQUJJd0FBQVFFQTg3OUJKR1lsUFRMSXVjOS9SNU1ZaU40eWMvWWlDTGNkQnBTZHpnSzlEdDBCa2ZlM3JTejVjUG00d21laGRFN0drVkZYckJKMllIcVBMdU0xeXgxQVV4SWVicHdsSWw5Zi9hVUhPdHM5ZVZuVmg0Tnp0UHkwaVNVL1N2MGIyT0RRUXZjeTJ2WWN1amxvcnNjbDhKakFnZldzTzNXNGlHRWU2UXdCcFZvbWNNRThJVTM1djVWYnlsTTlPUlFhNnd2Wk1WclBFQ0J2d0l0VFk4Y1BXSDNNR1ppSy83NGVIYlNMS0E0UFkzZ000R0hJNDUwTmllMTZ5Z2dFZzJhVFFmV0ExcnJ5OUpZV0VvSFM5cEoxZG5McVpVM2svOE9XZ3FKcmlsd1NvQzVyR2pncDkzaXUwSDhUNittRUhHUlFlODROazF5NWxFU1NXSWJuNlA2MzZCbDN1UT09IHlvdXJAZW1haWwuY29tIg==
secrets/test_secret: c3VwZXJzZWNyZXQ=
secrets/umlaut: w4TDpMOWw7bDnMO8
secrets/url: cG9zdGdyZXNxbDovL3Rlc3QtYXBwLWJhY2tlbmQuYXBwLXRlc3Quc3ZjLmNsdXN0ZXIubG9jYWw6NTQzMg==
secrets/username: c29tZS11c2Vy
secrets/var with spaces: c29tZS1zZWNyZXQ=
secrets/var+with+pluses: c29tZS1zZWNyZXQ=

`
	FetchAllBase64YamlContent = `"secrets/another_test_secret": "some-secret"
"secrets/encoded": "secret-value"
"secrets/json_object_secret": "\"{\"auths\":{\"someurl\":{\"auth\":\"sometoken=\"}}}\""
"secrets/password": "7H1SiSmYp@5Sw0rd"
"secrets/ssh_key": "\"ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA879BJGYlPTLIuc9/R5MYiN4yc/YiCLcdBpSdzgK9Dt0Bkfe3rSz5cPm4wmehdE7GkVFXrBJ2YHqPLuM1yx1AUxIebpwlIl9f/aUHOts9eVnVh4NztPy0iSU/Sv0b2ODQQvcy2vYcujlorscl8JjAgfWsO3W4iGEe6QwBpVomcME8IU35v5VbylM9ORQa6wvZMVrPECBvwItTY8cPWH3MGZiK/74eHbSLKA4PY3gM4GHI450Nie16yggEg2aTQfWA1rry9JYWEoHS9pJ1dnLqZU3k/8OWgqJrilwSoC5rGjgp93iu0H8T6+mEHGRQe84Nk1y5lESSWIbn6P636Bl3uQ== your@email.com\""
"secrets/test_secret": "supersecret"
"secrets/umlaut": "ÄäÖöÜü"
"secrets/url": "postgresql://test-app-backend.app-test.svc.cluster.local:5432"
"secrets/username": "some-user"
"secrets/var with spaces": "some-secret"
"secrets/var+with+pluses": "some-secret"`
)
