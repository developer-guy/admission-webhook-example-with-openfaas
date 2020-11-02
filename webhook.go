package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/openfaas-incubator/connector-sdk/types"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"net/http"
	"os"
	"time"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

type WebhookServer struct {
	server     *http.Server
	controller types.Controller
}

// Webhook Server parameters
type WhSvrParameters struct {
	port           int    // webhook server port
	certFile       string // path to the x509 certificate for https
	keyFile        string // path to the x509 private key matching `CertFile`
	sidecarCfgFile string // path to sidecar injector configuration file
}

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1beta1.AddToScheme(runtimeScheme)
}

// ResponseReceiver enables connector to receive results from the
// function invocation
type ResponseReceiver struct {
	AdmissionResponse *v1beta1.AdmissionResponse
}

// Response is triggered by the controller when a message is
// received from the function invocation
func (r *ResponseReceiver) Response(res types.InvokerResponse) {
	if res.Error != nil {
		glog.Errorf("tester got error: %s", res.Error.Error())
	} else {
		glog.Infof("tester got result: [%d] %s => %s (%d) bytes", res.Status, res.Topic, res.Function, len(*res.Body))
		var response v1beta1.AdmissionResponse
		err := json.Unmarshal(*res.Body, &response)
		if err != nil {
			glog.Errorf("tester got error: %s", res.Error.Error())
		}
		r.AdmissionResponse = &response
		glog.Infof("tester got result : allowed %t", r.AdmissionResponse.Allowed)
	}
}

// validate deployments and services
func (whsvr *WebhookServer) validate(ar *v1beta1.AdmissionReview) (*v1beta1.AdmissionResponse, error) {
	requestBytes, err := json.Marshal(ar.Request)
	if err != nil {
		return nil, err
	}

	functionTopic := os.Getenv("FUNCTION_TOPIC")
	glog.Infof("Function topic: %s", functionTopic)

	receiver := ResponseReceiver{AdmissionResponse: nil}
	whsvr.controller.Subscribe(&receiver)

	attempt := 0
	glog.Info("Invoking function")
	for {
		if attempt < 3 {
			whsvr.controller.Invoke(functionTopic, &requestBytes)
			if receiver.AdmissionResponse != nil {
				glog.Info("AdmissionResponse is full, break")
				break
			}
			glog.Info("AdmissionResponse is still nil, Invoking function again after 1sec")
			time.Sleep(time.Second)
			attempt += 1
		} else {
			return nil, errors.New("tried 3 times, backed off")
		}
	}

	return receiver.AdmissionResponse, nil
}

// Serve method for webhook server
func (whsvr *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		glog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		glog.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		glog.Errorf("Can't decode body: %v", err)
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		if r.URL.Path == "/validate" {
			admissionResponse, err = whsvr.validate(&ar)
			if err != nil {
				glog.Errorf("Can't validate body: %v", err)
				admissionResponse = &v1beta1.AdmissionResponse{
					Result: &metav1.Status{
						Message: err.Error(),
					},
				}
			}
		}
	}

	admissionReview := v1beta1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		glog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	glog.Infof("Ready to write reponse ...")
	if _, err := w.Write(resp); err != nil {
		glog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}
