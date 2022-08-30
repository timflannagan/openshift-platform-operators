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
	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
)

var (
	TypeApplied = "Applied"

	ReasonSourceFailed    = "SourceFailed"
	ReasonUnpackPending   = "ApplyPending"
	ReasonApplyFailed     = "ApplyFailed"
	ReasonApplySuccessful = "ApplySuccessful"
)

const (
	SourcedBundleAnnotation = "platform.openshift.io/sourced-bundle"
)

func GetDesiredBundle(po *platformv1alpha1.PlatformOperator) string {
	annotations := po.GetAnnotations()
	desiredBundle, ok := annotations[SourcedBundleAnnotation]
	if ok {
		return desiredBundle
	}
	return ""
}

func SetDesiredBundle(po *platformv1alpha1.PlatformOperator, desiredBundle string) {
	annotations := po.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[SourcedBundleAnnotation] = desiredBundle
	po.SetAnnotations(annotations)
}

func SetActiveBundleDeployment(po *platformv1alpha1.PlatformOperator, name string) {
	po.Status.ActiveBundleDeployment = platformv1alpha1.ActiveBundleDeployment{
		Name: name,
	}
}
