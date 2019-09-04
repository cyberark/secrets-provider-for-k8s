package log

/*
	This go file centralizes log messages (in different levels) so we have them all in one place.

	Although having the names of the consts as the error code (i.e CSPFK001E) and not as a descriptive name (i.e InvalidStoreType)
	can reduce readability of the code that raises the error, we decided to do so for the following reasons:
		1.  Improves supportability – when we get this code in the log we can find it directly in the code without going
			through the “log_messages.go” file first
		2. Validates we don’t have error code duplications – If the code is only in the string then 2 errors can have the
			same code (which is something that a developer can easily miss). However, If they are in the object name
			then the compiler will not allow it.
*/

// ERROR MESSAGES
const CSPFK001E string = "CSPFK001E Error creating the secret handler object"
const CSPFK002E string = "CSPFK002E Error creating the access token object"
const CSPFK003E string = "CSPFK003E Error creating the secrets config"
const CSPFK004E string = "CSPFK004E Error creating access token object"
const CSPFK005E string = "CSPFK005E Store type %s is invalid"
const CSPFK017E string = "CSPFK017E Environment variable '%s' must be provided"
const CSPFK018E string = "CSPFK018E Failed to load Conjur config. Reason: %s"
const CSPFK019E string = "CSPFK019E Failed to create Conjur client from token. Reason: %s"
const CSPFK020E string = "CSPFK020E Error creating Conjur secrets provider"
const CSPFK021E string = "CSPFK021E Error retrieving Conjur secrets. Reason: %s"
const CSPFK022E string = "CSPFK022E Failed to create k8s secrets handler"
const CSPFK023E string = "CSPFK023E Failure retrieving k8s secretsHandlerK8sUseCase"
const CSPFK024E string = "CSPFK024E Failed to retrieve access token"
const CSPFK025E string = "CSPFK025E Error parsing Conjur variable ids"
const CSPFK026E string = "CSPFK026E Error retrieving Conjur k8sSecretsHandler"
const CSPFK027E string = "CSPFK027E Failed to update K8s K8sSecretsHandler map"
const CSPFK028E string = "CSPFK028E Failed to patch K8s K8sSecretsHandler"
const CSPFK029E string = "CSPFK029E Error map should not be empty"
const CSPFK030E string = "CSPFK030E Failed to update k8s k8sSecretsHandler map"
const CSPFK031E string = "CSPFK031E Failed to parse Conjur variable ID: %s"
const CSPFK032E string = "CSPFK032E Error reading k8s secrets"
const CSPFK033E string = "CSPFK033E Failed to patch k8s secret"
const CSPFK034E string = "CSPFK034E Failed to load in-cluster Kubernetes client config. Reason: %s"
const CSPFK035E string = "CSPFK035E Failed to create Kubernetes client. Reason: %s"
const CSPFK036E string = "CSPFK036E Failed to retrieve Kubernetes secret. Reason: %s"
const CSPFK037E string = "CSPFK037E Failed to parse Kubernetes secret list"
const CSPFK038E string = "CSPFK038E Failed to patch Kubernetes secret. Reason: %s"
const CSPFK039E string = "CSPFK039E Data entry map cannot be empty"
const CSPFK042E string = "CSPFK042E Provided incorrect value for environment variable %s"
const CSPFK045E string = "CSPFK045E Failed to instantiate authenticator configuration"
const CSPFK046E string = "CSPFK046E Failed to instantiate storage configuration"
const CSPFK047E string = "CSPFK047E Setting SECRETS_DESTINATION environment variable to 'k8s_secrets' must run as init container"
const CSPFK048E string = "CSPFK048E Failed to instantiate storage handler"
const CSPFK049E string = "CSPFK049E Failed to instantiate authenticator object"
const CSPFK050E string = "CSPFK050E Failure authenticating"
const CSPFK051E string = "CSPFK051E Failure parsing authentication response"
const CSPFK052E string = "CSPFK052E Failed to handle secrets"
const CSPFK053E string = "CSPFK053E Retransmission backoff exhausted"
const CSPFK054E string = "CSPFK054E Failed to delete access token"
const CSPFK065E string = "CSPFK065E AccessToken failed to delete access token. Reason: %s"
const CSPFK066E string = "CSPFK066E Failed to find any k8s secrets defined with a '%s’ data entry"
const CSPFK067E string = "CSPFK067E k8s secret '%s' has no value defined for the '%s' data entry"
const CSPFK068E string = "CSPFK068E k8s secret '%s' has an invalid value for '%s' data entry"
const CSPFK069E string = "CSPFK069E Failed to create conjur secrets retriever"

// INFO MESSAGES
const CSPFK001I string = "CSPFK001I Storage configuration is %s"
const CSPFK013I string = "CSPFK013I Waiting for %s to re-authenticate and fetch secrets."
const CSPFK014I string = "CSPFK014I Creating Kubernetes client..."
const CSPFK015I string = "CSPFK015I Creating Conjur client..."
const CSPFK016I string = "CSPFK016I Retrieving Kubernetes secret '%s' from namespace '%s'..."
const CSPFK017I string = "CSPFK017I Patching Kubernetes secret '%s' in namespace '%s'"
const CSPFK018I string = "CSPFK018I Retrieving following secrets from Conjur: "
const CSPFK019I string = "CSPFK019I Authenticating as user '%s'"
