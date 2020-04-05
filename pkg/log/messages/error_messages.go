package messages

/*
	This go file centralizes error log messages so we have them all in one place.

	Although having the names of the consts as the error code (i.e CSPFK014E) and not as a descriptive name (i.e InvalidStoreType)
	can reduce readability of the code that raises the error, we decided to do so for the following reasons:
		1.  Improves supportability – when we get this code in the log we can find it directly in the code without going
			through the “info_messages.go” file first
		2. Validates we don’t have error code duplications – If the code is only in the string then 2 errors can have the
			same code (which is something that a developer can easily miss). However, If they are in the object name
			then the compiler will not allow it.
*/

// Access Token
const CSPFK001E string = "CSPFK001E Failed to create access token object"
const CSPFK002E string = "CSPFK002E Failed to retrieve access token"
const CSPFK003E string = "CSPFK003E AccessToken failed to delete access token data. Reason: %s"

// Environment variables
const CSPFK004E string = "CSPFK004E Environment variable '%s' must be provided"
const CSPFK005E string = "CSPFK005E Provided incorrect value for environment variable %s"
const CSPFK007E string = "CSPFK007E Setting SECRETS_DESTINATION environment variable to 'k8s_secrets' must run as init container"

// Authenticator
const CSPFK008E string = "CSPFK008E Failed to instantiate authenticator configuration"
const CSPFK009E string = "CSPFK009E Failed to instantiate authenticator object"
const CSPFK010E string = "CSPFK010E Failed to authenticate"
const CSPFK011E string = "CSPFK011E Failed to parse authentication response"

// ProvideConjurSecrets
const CSPFK014E string = "CSPFK014E Failed to instantiate ProvideConjurSecrets function. Reason: %s"
const CSPFK015E string = "CSPFK015E Failed to instantiate secrets config"
const CSPFK016E string = "CSPFK016E Failed to provide Conjur secrets"

// Kubernetes
const CSPFK018E string = "CSPFK018E Failed to create Kubernetes client"
const CSPFK019E string = "CSPFK019E Failed to load in-cluster Kubernetes client config"
const CSPFK020E string = "CSPFK020E Failed to retrieve k8s secret"
const CSPFK021E string = "CSPFK021E Failed to retrieve k8s secrets"
const CSPFK022E string = "CSPFK022E Failed to update k8s secret"
const CSPFK023E string = "CSPFK023E Failed to update K8s secrets"
const CSPFK025E string = "CSPFK025E PathMap cannot be empty"
const CSPFK027E string = "CSPFK027E Failed to update K8s secrets map with Conjur secrets"
const CSPFK028E string = "CSPFK028E Unable to update k8s secret '%s'"

// Conjur
const CSPFK031E string = "CSPFK031E Failed to load Conjur config. Reason: %s"
const CSPFK032E string = "CSPFK032E Failed to create Conjur client from token. Reason: %s"
const CSPFK033E string = "CSPFK033E Failed to create Conjur client"
const CSPFK034E string = "CSPFK034E Failed to retrieve Conjur secrets. Reason: %s"
const CSPFK035E string = "CSPFK035E Failed to parse Conjur variable ID"
const CSPFK036E string = "CSPFK036E Variable ID '%s' is not in the format '<account>:variable:<variable_id>'"
const CSPFK037E string = "CSPFK037E Failed to parse Conjur variable IDs"

// General
const CSPFK038E string = "CSPFK038E Retransmission backoff exhausted"
