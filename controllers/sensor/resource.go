/*
Copyright 2019 - 2020 IBM, Inc.

Copyright 2018 BlackRock, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
	http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sensor

import (
	"github.com/argoproj/argo-events/common"
	controllerscommon "github.com/argoproj/argo-events/controllers/common"
	pc "github.com/argoproj/argo-events/pkg/apis/common"
	"github.com/argoproj/argo-events/pkg/apis/sensor/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/intstr"

		
	serving_v1alpha1_api "knative.dev/serving/pkg/apis/serving/v1alpha1"
	
	
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"fmt"
)

type sResourceCtx struct {
	// s is the gateway-controller object
	s *v1alpha1.Sensor
	// reference to the gateway-controller-controller
	controller *SensorController

	controllerscommon.ChildResourceContext
}

// NewSensorResourceContext returns new sResourceCtx
func NewSensorResourceContext(s *v1alpha1.Sensor, controller *SensorController) sResourceCtx {
	return sResourceCtx{
		s:          s,
		controller: controller,
		ChildResourceContext: controllerscommon.ChildResourceContext{
			SchemaGroupVersionKind:            v1alpha1.SchemaGroupVersionKind,
			LabelOwnerName:                    common.LabelSensorName,
			LabelKeyOwnerControllerInstanceID: common.LabelKeySensorControllerInstanceID,
			AnnotationOwnerResourceHashName:   common.AnnotationSensorResourceSpecHashName,
			InstanceID:                        controller.Config.InstanceID,
		},
	}
}

// sensorResourceLabelSelector returns label selector of the sensor of the context
func (src *sResourceCtx) sensorResourceLabelSelector() (labels.Selector, error) {
	req, err := labels.NewRequirement(common.LabelSensorName, selection.Equals, []string{src.s.Name})
	if err != nil {
		return nil, err
	}
	return labels.NewSelector().Add(*req), nil
}

// createSensorService creates a service
func (src *sResourceCtx) createSensorService(svc *corev1.Service) (*corev1.Service, error) {
	return src.controller.kubeClientset.CoreV1().Services(src.s.Namespace).Create(svc)
}

// TODO: obviously we shouldn't create client each time. this client should be moved to cache same as regular
// func (src *sResourceCtx) createKnativeClient() *serving_v1alpha1_client.ServingV1alpha1Client {
// 	kn_config := serving_v1alpha1_client.NewForConfigOrDie(src.controller.kubeConfig)
// 	return kn_config
// }

func (src *sResourceCtx) createKnativeSensorService(service *serving_v1alpha1_api.Service) (*serving_v1alpha1_api.Service, error) {
	_, err := src.controller.knClient.Services(src.s.Namespace).Create(service)
	if err != nil {
		return nil, err
	}

	return service, nil
}

// deleteSensorService deletes a given service
func (src *sResourceCtx) deleteSensorService(svc *corev1.Service) error {
	return src.controller.kubeClientset.CoreV1().Services(src.s.Namespace).Delete(svc.Name, &metav1.DeleteOptions{})
}

// deleteKnativeSensorService deletes given Knative service
func (src *sResourceCtx) deleteKnativeSensorService(svc *serving_v1alpha1_api.Service) error {
	serviceName := src.s.Spec.KnativeService.Name
	if serviceName == "" {
		serviceName = common.DefaultServiceName(src.s.Name)
	}
	return src.controller.knClient.Services(src.s.Namespace).Delete(
		serviceName,
		&v1.DeleteOptions{},
	)
}

// getSensorService returns the service of sensor
func (src *sResourceCtx) getSensorService() (*corev1.Service, error) {
	selector, err := src.sensorResourceLabelSelector()
	if err != nil {
		return nil, err
	}
	svcs, err := src.controller.svcInformer.Lister().Services(src.s.Namespace).List(selector)
	if err != nil {
		return nil, err
	}
	if len(svcs) == 0 {
		return nil, nil
	}
	return svcs[0], nil
}

func (src *sResourceCtx) getKnativeSensorService() (*serving_v1alpha1_api.Service, error) {
	serviceName := src.s.Spec.KnativeService.Name
	if serviceName == "" {
		serviceName = common.DefaultServiceName(src.s.Name)
	}

	service, err := src.controller.knClient.Services(src.s.Namespace).Get(serviceName, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return service, nil
}

// newKnativeSensorService returns a new Knative service that exposes sensor.
func (src *sResourceCtx) newKnativeSensorService() (*serving_v1alpha1_api.Service, error) {
	fmt.Println(">>>> In newKnativeSensorService")
	knServiceTemplateSpec := src.s.Spec.KnativeService.DeepCopy()
	if knServiceTemplateSpec == nil {
			return nil, nil
	}

	service := &serving_v1alpha1_api.Service{
		ObjectMeta: knServiceTemplateSpec.ObjectMeta,
		Spec: serving_v1alpha1_api.ServiceSpec{
			ConfigurationSpec: serving_v1alpha1_api.ConfigurationSpec{
				Template: &knServiceTemplateSpec.Template,
			},
		},
	}

	src.SetObjectMeta(src.s, service)
	
	if service.Name == "" {
		service.Name = common.DefaultServiceName(src.s.Name)
	}

	src.setupContainersEnv(service.Spec.Template.Spec.Containers)
	return service, nil
}

// newSensorService returns a new service that exposes sensor.
func (src *sResourceCtx) newSensorService() (*corev1.Service, error) {
	serviceTemplateSpec := src.getServiceTemplateSpec()
	if serviceTemplateSpec == nil {
		return nil, nil
	}
	service := &corev1.Service{
		ObjectMeta: serviceTemplateSpec.ObjectMeta,
		Spec:       serviceTemplateSpec.Spec,
	}
	if service.Namespace == "" {
		service.Namespace = src.s.Namespace
	}
	if service.Name == "" {
		service.Name = common.DefaultServiceName(src.s.Name)
	}
	err := src.SetObjectMeta(src.s, service)
	return service, err
}

// getSensorPod returns the pod of sensor
func (src *sResourceCtx) getSensorPod() (*corev1.Pod, error) {
	selector, err := src.sensorResourceLabelSelector()
	if err != nil {
		return nil, err
	}
	pods, err := src.controller.podInformer.Lister().Pods(src.s.Namespace).List(selector)
	if err != nil {
		return nil, err
	}
	if len(pods) == 0 {
		return nil, nil
	}
	return pods[0], nil
}

// createSensorPod creates a pod of sensor
func (src *sResourceCtx) createSensorPod(pod *corev1.Pod) (*corev1.Pod, error) {
	return src.controller.kubeClientset.CoreV1().Pods(src.s.Namespace).Create(pod)
}

// deleteSensorPod deletes a given pod
func (src *sResourceCtx) deleteSensorPod(pod *corev1.Pod) error {
	return src.controller.kubeClientset.CoreV1().Pods(src.s.Namespace).Delete(pod.Name, &metav1.DeleteOptions{})
}

// newSensorPod returns a new pod of sensor
func (src *sResourceCtx) newSensorPod() (*corev1.Pod, error) {
	podTemplateSpec := src.s.Spec.Template.DeepCopy()
	pod := &corev1.Pod{
		ObjectMeta: podTemplateSpec.ObjectMeta,
		Spec:       podTemplateSpec.Spec,
	}
	if pod.Namespace == "" {
		pod.Namespace = src.s.Namespace
	}
	if pod.Name == "" {
		pod.Name = src.s.Name
	}
	src.setupContainersForSensorPod(pod)
	err := src.SetObjectMeta(src.s, pod)
	return pod, err
}

// containers required for sensor deployment
func (src *sResourceCtx) setupContainersEnv(containers []corev1.Container) {
	// env variables
	envVars := []corev1.EnvVar{
		{
			Name:  common.SensorName,
			Value: src.s.Name,
		},
		{
			Name:  common.SensorNamespace,
			Value: src.s.Namespace,
		},
		{
			Name:  common.EnvVarSensorControllerInstanceID,
			Value: src.controller.Config.InstanceID,
		},
	}
	for i, _ := range containers {
		containers[i].Env = append(containers[i].Env, envVars...)
	}
}

// containers required for sensor deployment
func (src *sResourceCtx) setupContainersForSensorPod(pod *corev1.Pod) {
	// env variables
	envVars := []corev1.EnvVar{
		{
			Name:  common.SensorName,
			Value: src.s.Name,
		},
		{
			Name:  common.SensorNamespace,
			Value: src.s.Namespace,
		},
		{
			Name:  common.EnvVarSensorControllerInstanceID,
			Value: src.controller.Config.InstanceID,
		},
	}
	for i, container := range pod.Spec.Containers {
		container.Env = append(container.Env, envVars...)
		pod.Spec.Containers[i] = container
	}
}

func (src *sResourceCtx) getServiceTemplateSpec() *pc.ServiceTemplateSpec {
	var serviceSpec *pc.ServiceTemplateSpec
	// Create a ClusterIP service to expose sensor in cluster if the event protocol type is HTTP
	if src.s.Spec.EventProtocol.Type == pc.HTTP {
		serviceSpec = &pc.ServiceTemplateSpec{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:       intstr.Parse(src.s.Spec.EventProtocol.Http.Port).IntVal,
						TargetPort: intstr.FromInt(int(intstr.Parse(src.s.Spec.EventProtocol.Http.Port).IntVal)),
					},
				},
				Type: corev1.ServiceTypeClusterIP,
				Selector: map[string]string{
					common.LabelSensorName:                    src.s.Name,
					common.LabelKeySensorControllerInstanceID: src.controller.Config.InstanceID,
				},
			},
		}
	}
	return serviceSpec
}
