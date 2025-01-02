package k8swebhooks

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	k8sSecretsStorage "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_secrets_storage"
	"io/ioutil"
	admission "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"net/http"
	"slices"
)

type MutateFunc func(v1.Secret) (patchData map[string][]byte, err error)

type WebhookServer struct {
	server            *http.Server
	mutateFunc        MutateFunc
	provider          k8sSecretsStorage.K8sProvider
	interestedSecrets []string
}

// Webhook Server parameters
type ServerParams struct {
	port           int    // webhook server port
	certFile       string // path to the x509 certificate for https
	keyFile        string // path to the x509 private key matching `CertFile`
	sidecarCfgFile string // path to sidecar injector configuration file
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func StartWebhookServer(provider k8sSecretsStorage.K8sProvider, interestedSecrets []string) {
	whsvr := initWebhookServer(provider, interestedSecrets)
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", whsvr.serve)
	whsvr.server.Handler = mux
	go func() {
		log.Info("Starting admission webhook server...")
		err := whsvr.server.ListenAndServeTLS("", "")
		if err != nil {
			log.Error(messages.CSPFK072E, fmt.Sprintf("server failed to start: %s", err.Error()))
			log.Warn(messages.CSPFK072E, "secret mutation will not work")
		}
	}()
}

func initWebhookServer(provider k8sSecretsStorage.K8sProvider, interestedSecrets []string) WebhookServer {

	pair, err := tls.LoadX509KeyPair("/etc/webhook/certs/cert.pem", "/etc/webhook/certs/key.pem")
	if err != nil {
		log.Error(messages.CSPFK072E, "failed to load key pair: %v", err)
	}

	return WebhookServer{
		server: &http.Server{
			Addr:      fmt.Sprintf(":%v", 5000),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},
		mutateFunc:        provider.Mutate,
		provider:          provider,
		interestedSecrets: interestedSecrets,
	}
}

func (whsvr *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {

	body, err := requestBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	admissionReview := admission.AdmissionReview{}
	admissionReview.TypeMeta.Kind = "AdmissionReview"
	admissionReview.TypeMeta.APIVersion = "admission.k8s.io/v1"
	admissionReview.Response = &admission.AdmissionResponse{
		Result:  &metav1.Status{},
		Allowed: true,
	}

	defer writeResponse(w, admissionReview)

	ar := admission.AdmissionReview{}
	if _, _, err := serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer().Decode(body, nil, &ar); err != nil {
		log.Error(messages.CSPFK071E, "", "can't decode body: %v", err)
		admissionReview.Response.Result.Message = err.Error()
		return
	} else {
		admissionReview.Response.UID = ar.Request.UID
	}

	var secret v1.Secret
	var patchData map[string][]byte
	if ar.Request.Operation == admission.Delete {
		whsvr.provider.Delete([]string{ar.Request.Name})
		return
	}

	var patch []patchOperation
	muatatePatchType := admission.PatchTypeJSONPatch
	var patchSecretData map[string]string

	if err := json.Unmarshal(ar.Request.Object.Raw, &secret); err != nil {
		log.Error(messages.CSPFK071E, "", err.Error())
		admissionReview.Response.Result.Message = err.Error()
		return
	} else {

		// check if k8s Secret is in our interest
		// TODO in future the webhook might have selector only on labeled secret. The interestedSecrets check should be removed then
		if !slices.Contains(whsvr.interestedSecrets, secret.Name) {
			return
		}

		//first check for 'magic' annotation
		if secret.Annotations["conjur.org/just-provided"] != "" {
			//it means the hooks is triggered by provide operation
			//remove the annotation and skip out
			patch = append(patch, patchOperation{
				Op:   "remove",
				Path: "/metadata/annotations/conjur.org~1just-provided",
			})
			goto response
		}
	}

	patchData, err = whsvr.provider.Mutate(secret)
	if secret.Annotations == nil {
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  "/metadata/annotations",
			Value: map[string]string{},
		})
	}
	if err != nil {
		log.Error(messages.CSPFK071E, secret.Name, err.Error())
		admissionReview.Response.Result.Message = err.Error()
		return
	}

	patchSecretData = make(map[string]string)
	for itemName, secretValue := range patchData {
		patchSecretData[itemName] = string(secretValue)
	}
	patch = append(patch, patchOperation{
		Op:    "add",
		Path:  "/stringData",
		Value: patchSecretData,
	})

response:
	admissionReview.Response.PatchType = &muatatePatchType
	res, _ := json.Marshal(patch)
	admissionReview.Response.Patch = res
}

func requestBody(r *http.Request) ([]byte, error) {
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			if len(data) == 0 {
				return nil, fmt.Errorf("empty body")
			}
			return data, nil
		} else {
			return nil, err
		}
	}
	return nil, fmt.Errorf("request without body")
}

func writeResponse(w http.ResponseWriter, admissionReview admission.AdmissionReview) {
	if resp, err := json.Marshal(admissionReview); err != nil {
		log.Error(messages.CSPFK071E, "", "can't encode response to JSON: %v", err)

		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	} else {
		if _, err := w.Write(resp); err != nil {
			log.Error(messages.CSPFK071E, "", "Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
	}
}
