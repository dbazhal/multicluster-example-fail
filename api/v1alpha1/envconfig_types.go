/*
Copyright 2022.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	envc "git.company.tld/platform/operator-environment/controllers"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type Environment struct {
	// Used to generate environment object name
	Name string `json:"name"`
	// dev / testing / prod
	EnvClass string `json:"env_class"`
	// Cluster api url
	ApiUrl string `json:"api_url"`
	// Namespace in selected cluster
	Namespace     string   `json:"namespace"`
	NetworkPolicy bool     `json:"network_policy,omitempty"`
	Telepresence  bool     `json:"enable_telepresence_anyuid,omitempty"`
	Labels        []string `json:"labels,omitempty"`
}

type Role struct {
	// +kubebuilder:validation:Enum=admin;edit;view
	Name   string   `json:"role"`
	Groups []string `json:"groups"`
}

type EnvRole struct {
	EnvClass string `json:"env"`
	Roles    []Role `json:"roles"`
}

// Important: Run "make" to regenerate code after modifying this file

// EnvconfigSpec defines the desired state of Envconfig
// +kubebuilder:pruning:PreserveUnknownFields
type EnvconfigSpec struct {
	// +kubebuilder:validation:MinItems=1
	Environments []Environment `json:"environments"`
	// +kubebuilder:validation:MinItems=1
	IsDefault   bool       `json:"is_default"`
	Owners      []string   `json:"owners"`
	EnvSequence [][]string `json:"environments_sequence,omitempty"`
	Product     string     `json:"product,omitempty"`
	DisplayName string     `json:"displayName,omitempty"`
	Description string     `json:"description,omitempty"`
	Roles       []Role     `json:"roles,omitempty"`
	RolesEnv    []EnvRole  `json:"roles_env,omitempty"`
}

type Cond struct {
	// +kubebuilder:validation:Enum=Available;Tokensecret
	Type string `json:"Type"`
	// +kubebuilder:validation:Enum=True;False;Unknown
	Status string `json:"Status"`
	Message string `json:"Message,omitempty"`
	Reason string `json:"Reason,omitempty"`
}

// EnvconfigStatus defines the observed state of Envconfig
// +kubebuilder:pruning:PreserveUnknownFields
type EnvconfigStatus struct {
	Error               string `json:"error,omitempty"`
	AllEnvsCreated      bool   `json:"allEnvironmentsCreated,omitempty"`
	ApiTokensReady      bool   `json:"apiTokensReady,omitempty"`
	ApiTokensSecret     string `json:"apiTokensSecret,omitempty"`
	ImagePullerInjected bool   `json:"imagePullerInjected,omitempty"`
	Conditions          []Cond `json:"conditions,omitempty"`
	ConsoleUrls         map[string]string  `json:"console_urls,omitempty"`
}

// Important: Run "make" to regenerate code after modifying this file

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Envconfig is the Schema for the envconfigs API
// +kubebuilder:resource:path=envconfigs,shortName=envc
type Envconfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvconfigSpec   `json:"spec,omitempty"`
	Status EnvconfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EnvconfigList contains a list of Envconfig
type EnvconfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Envconfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Envconfig{}, &EnvconfigList{})
}

func (in Envconfig) GetLinkLabels() map[string]string {
	return map[string]string{envc.UnitProjectLabel: in.Namespace, envc.EnvConfigLabel: in.Name}
}

func (in Envconfig) GetControlApiUrl() string {
	// danger! costy method :D
	for _, envItem := range in.Spec.Environments {
		if envItem.EnvClass == "infra" {
			return envItem.ApiUrl
		}
	}
	return ""
}

// for later - https://sdk.operatorframework.io/docs/building-operators/golang/advanced-topics/#manage-cr-status-conditions if required
// https://pkg.go.dev/github.com/operator-framework/operator-sdk/pkg/status?utm_source=godoc#Conditions.SetCondition
