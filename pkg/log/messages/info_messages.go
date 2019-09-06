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

const CSPFK102I string = "CSPFK102I Authenticating as user '%s'"
const CSPFK103I string = "CSPFK103I Creating Conjur client..."
const CSPFK104I string = "CSPFK104I Retrieving following secrets from Conjur: "
const CSPFK105I string = "CSPFK105I Creating Kubernetes client..."
const CSPFK106I string = "CSPFK106I Retrieving Kubernetes secret '%s' from namespace '%s'..."
const CSPFK107I string = "CSPFK107I Patching Kubernetes secret '%s' in namespace '%s'"
const CSPFK108I string = "CSPFK108I Waiting for %s to re-authenticate and fetch secrets."
