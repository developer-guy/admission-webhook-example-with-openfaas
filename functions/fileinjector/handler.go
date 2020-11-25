package function

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"net/http"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/admission/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Patches []Patch

type Patch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func Handle(w http.ResponseWriter, r *http.Request) {
	response := &v1beta1.AdmissionResponse{
		Allowed: true,
	}

	var input []byte

	if r.Body != nil {
		defer r.Body.Close()

		body, _ := ioutil.ReadAll(r.Body)

		input = body
	}

	var req v1beta1.AdmissionRequest
	if err := json.Unmarshal(input, &req); err != nil {
		log.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		return
	}

	switch req.Kind.Kind {
	case "Deployment":
		var deployment appsv1.Deployment
		if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
			log.Errorf("Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
			return
		}

		log.Infof("AdmissionReview for Deployment: Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
			req.Kind, deployment.Namespace, req.Name, deployment.Name, req.UID, req.Operation, req.UserInfo)

		// create corresponding json patch schemas
		patches := &Patches{}

		//TODO: check init container

		memoryRequest, _ := resource.ParseQuantity("50Mi")
		cpuRequest, _ := resource.ParseQuantity("100Mi")

		memoryLimit, _ := resource.ParseQuantity("75Mi")
		cpuLimit, _ := resource.ParseQuantity("125Mi")

		resources := corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    cpuLimit,
				corev1.ResourceMemory: memoryLimit,
			},

			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    cpuRequest,
				corev1.ResourceMemory: memoryRequest,
			},
		}

		for i, c := range deployment.Spec.Template.Spec.Containers {
			r, l := c.Resources.Requests, c.Resources.Limits

			if len(r) == 0 && len(l) == 0 {
				patches.addPatch(i, resources)
			}
		}

		patchesBytes, err := json.Marshal(patches)
		if err != nil {
			log.Errorf("Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
			return
		}

		patchType := v1beta1.PatchTypeJSONPatch
		response.Patch = patchesBytes
		response.PatchType = &patchType
		response.Result = &metav1.Status{
			Status: metav1.StatusSuccess,
			Code:   http.StatusOK,
		}
	}

	responseBytes, _ := json.Marshal(response)
	if _, err := w.Write(responseBytes); err != nil {
		log.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		return
	}

}

func (p *Patches) addPatch(index int, resources corev1.ResourceRequirements) {
	*p = append(*p, Patch{
		Op:    "add",
		Path:  fmt.Sprintf("/spec/template/spec/containers/%d/resources", index),
		Value: resources,
	})
}
