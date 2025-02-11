package messages

/*
	This go file centralizes info log messages so we have them all in one place.

	Although having the names of the consts as the error code (i.e CSPFK014E) and not as a descriptive name (i.e InvalidStoreType)
	can reduce readability of the code that raises the error, we decided to do so for the following reasons:
		1.  Improves supportability – when we get this code in the log we can find it directly in the code without going
			through the “info_messages.go” file first
		2. Validates we don’t have error code duplications – If the code is only in the string then 2 errors can have the
			same code (which is something that a developer can easily miss). However, If they are in the object name
			then the compiler will not allow it.
*/

const CSPFK002I string = "CSPFK002I Creating DAP/Conjur client"
const CSPFK003I string = "CSPFK003I Retrieving following secrets from DAP/Conjur: %v"
const CSPFK004I string = "CSPFK004I Creating Kubernetes client"
const CSPFK005I string = "CSPFK005I Retrieving Kubernetes secret '%s' from namespace '%s'"
const CSPFK006I string = "CSPFK006I Updating Kubernetes secret '%s' in namespace '%s'"
const CSPFK008I string = "CSPFK008I CyberArk Secrets Provider for Kubernetes v%s starting up"
const CSPFK009I string = "CSPFK009I DAP/Conjur Secrets updated in Kubernetes successfully"
const CSPFK010I string = "CSPFK010I Updating Kubernetes Secrets: %d retries out of %d"
const CSPFK011I string = "CSPFK011I Annotation '%s' valid, but not recognized"
const CSPFK012I string = "CSPFK012I Secrets Provider setting '%s' set by both environment variable '%s' and annotation '%s'"
const CSPFK014I string = "CSPFK014I Authenticator setting %s provided by %s"
const CSPFK015I string = "CSPFK015I DAP/Conjur Secrets pushed to shared volume successfully"
const CSPFK017I string = "CSPFK017I Creating default file name for secret group '%s'"
const CSPFK018I string = "CSPFK018I No change in secret file, no secret files written"
const CSPFK019I string = "CSPFK019I Error fetching secrets, deleting secrets file"
const CSPFK020I string = "CSPFK020I No change in Kubernetes secret, no secrets updated"
const CSPFK021I string = "CSPFK021I Error fetching Conjur secrets, clearing Kubernetes secrets"
const CSPFK022I string = "CSPFK022I Storing secret with base64 content-type '%s' in destination '%s'"
const CSPFK023I string = "CSPFK023I Retrieving all available secrets from Conjur"
const CSPFK024I string = "CSPFK024I Trigger periodic provider"
const CSPFK025I string = "CSPFK025I Refresh interval set to %s"
