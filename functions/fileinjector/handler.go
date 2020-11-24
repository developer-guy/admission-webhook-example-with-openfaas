package function

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
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
		memoryLimit, _ := resource.ParseQuantity("75Mi")
		cpuRequest, _ := resource.ParseQuantity("100Mi")
		cpuLimit, _ := resource.ParseQuantity("125Mi")

		for i, c := range deployment.Spec.Template.Spec.Containers {
			r, l := c.Resources.Requests, c.Resources.Limits

			//Requests
			if r.Memory() == nil {
				patches.addPatch(i, memoryRequest, "requests", "memory")
			}
			if r.Cpu() == nil {
				patches.addPatch(i, cpuRequest, "requests", "cpu")
			}

			//Limits
			if l.Memory() == nil {
				patches.addPatch(i, memoryLimit, "limits", "memory")
			}
			if l.Cpu() == nil {
				patches.addPatch(i, cpuLimit, "limits", "cpu")
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

func (p *Patches) addPatch(index int, request resource.Quantity, resType, specType string) {
	*p = append(*p, Patch{
		Op:    "add",
		Path:  fmt.Sprintf("/spec/template/spec/containers/%d/resources/%s/%s", index, resType, specType),
		Value: request,
	})
}

func (p *Patches) addVolumes(pod *corev1.Pod, volumes []corev1.Volume) {
	first := len(pod.Spec.Volumes) == 0
	path := "/spec/volumes"
	var value interface{}
	for _, v := range volumes {
		value = v
		tempPath := path
		if first {
			first = false
			value = []corev1.Volume{v}
		} else {
			tempPath = path + "/-"
		}

		*p = append(*p, Patch{
			Op:    "add",
			Path:  tempPath,
			Value: value,
		})
	}
}

func (p *Patches) addVolumeMounts(pod *corev1.Pod, vms []corev1.VolumeMount) {
	first := len(pod.Spec.Containers[0].VolumeMounts) == 0
	path := "/spec/containers/0/volumeMounts"
	var value interface{}
	for _, vm := range vms {
		value = vm
		tempPath := path
		if first {
			first = false
			value = []corev1.VolumeMount{vm}
		} else {
			tempPath = path + "/-"
		}

		*p = append(*p, Patch{
			Op:    "add",
			Path:  tempPath,
			Value: value,
		})
	}
}
