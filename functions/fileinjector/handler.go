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
	case "Pod":
		var pod corev1.Pod
		if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
			log.Errorf("Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
			return
		}

		log.Infof("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
			req.Kind, pod.Namespace, req.Name, pod.Name, req.UID, req.Operation, req.UserInfo)

		// add the volume to the pod
		configMapVolume := corev1.Volume{
			Name: "hello-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "hello-configmap",
					},
				},
			},
		}

		// add the volumeMount to the pod that mounted to configMap Volume
		configMapVolumeMount := corev1.VolumeMount{
			Name:      "hello-volume",
			ReadOnly:  true,
			MountPath: "/etc/config",
		}

		// create corresponding json patch schemas
		jsonPatches := &Patches{}
		jsonPatches.addVolumes(&pod, []corev1.Volume{configMapVolume})
		jsonPatches.addVolumeMounts(&pod, []corev1.VolumeMount{configMapVolumeMount})

		patchesBytes, err := json.Marshal(jsonPatches)
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
