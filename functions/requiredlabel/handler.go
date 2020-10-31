package function

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"strings"
)

const (
	nameLabel                             = "app.kubernetes.io/name"
	instanceLabel                         = "app.kubernetes.io/instance"
	versionLabel                          = "app.kubernetes.io/version"
	componentLabel                        = "app.kubernetes.io/component"
	partOfLabel                           = "app.kubernetes.io/part-of"
	managedByLabel                        = "app.kubernetes.io/managed-by"
	admissionWebhookAnnotationValidateKey = "admission-webhook-example.qikqiak.com/validate"
)

var (
	ignoredNamespaces = []string{
		metav1.NamespaceSystem,
		metav1.NamespacePublic,
	}

	requiredLabels = []string{
		nameLabel,
		instanceLabel,
		versionLabel,
		componentLabel,
		partOfLabel,
		managedByLabel,
	}
)

func admissionRequired(ignoredList []string, admissionAnnotationKey string, metadata *metav1.ObjectMeta) bool {
	// skip special kubernetes system namespaces
	for _, namespace := range ignoredList {
		if metadata.Namespace == namespace {
			log.Infof("Skip validation for %v for it's in special namespace:%v", metadata.Name, metadata.Namespace)
			return false
		}
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	var required bool
	switch strings.ToLower(annotations[admissionAnnotationKey]) {
	default:
		required = true
	case "n", "no", "false", "off":
		required = false
	}
	return required
}

func validationRequired(ignoredList []string, metadata *metav1.ObjectMeta) bool {
	required := admissionRequired(ignoredList, admissionWebhookAnnotationValidateKey, metadata)
	log.Infof("Validation policy for %v/%v: required:%v", metadata.Namespace, metadata.Name, required)
	return required
}

func Handle(w http.ResponseWriter, r *http.Request) {
	var (
		availableLabels                 map[string]string
		objectMeta                      *metav1.ObjectMeta
		resourceNamespace, resourceName string
	)

	var input []byte

	if r.Body != nil {
		defer r.Body.Close()

		body, _ := ioutil.ReadAll(r.Body)

		input = body
	}

	response := &v1beta1.AdmissionResponse{
		Allowed: true,
	}

	var req v1beta1.AdmissionRequest
	if err := json.Unmarshal(input, &req); err != nil {
		log.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}

	log.Infof("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, resourceName, req.UID, req.Operation, req.UserInfo)

	switch req.Kind.Kind {
	case "Pod":
		var pod corev1.Pod
		if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
			log.Errorf("Could not unmarshal raw object: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
		resourceName, resourceNamespace, objectMeta = pod.Name, pod.Namespace, &pod.ObjectMeta
		availableLabels = pod.Labels
	}

	if !validationRequired(ignoredNamespaces, objectMeta) {
		log.Infof("Skipping validation for %s/%s due to policy check", resourceNamespace, resourceName)
		w.WriteHeader(http.StatusOK)
		responseBytes, _ := json.Marshal(response)

		if _, err := w.Write(responseBytes); err != nil {
			log.Errorf("Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
	}

	allowed := true
	var result *metav1.Status
	log.Info("available labels:", availableLabels)
	log.Info("required labels", requiredLabels)
	for _, rl := range requiredLabels {
		if _, ok := availableLabels[rl]; !ok {
			allowed = false
			result = &metav1.Status{
				Reason: "required labels are not set",
			}
			break
		}
	}

	response.Allowed = allowed
	response.Result = result

	responseBytes, _ := json.Marshal(response)
	if _, err := w.Write(responseBytes); err != nil {
		log.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}

}
