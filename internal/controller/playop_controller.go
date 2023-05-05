/*
Copyright 2023.

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

package controller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/example/play-operator/api/v1alpha1"
)

// Definitions to manage status conditions
const (
	// typeAvailablePlayOp represents the status of the Deployment reconciliation
	typeAvailablePlayOp = "Available"
)

// PlayOpReconciler reconciles a PlayOp object
type PlayOpReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cache.example.com,resources=playops,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cache.example.com,resources=playops/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cache.example.com,resources=playops/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *PlayOpReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the PlayOp instance
	// The purpose is check if the Custom Resource for the Kind PlayOp
	// is applied on the cluster if not we return nil to stop the reconciliation
	playop := &cachev1alpha1.PlayOp{}
	err := r.Get(ctx, req.NamespacedName, playop)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("playop resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get playop")
		return ctrl.Result{}, err
	}

	// Set the status as Unknown when no status are available
	if playop.Status.Conditions == nil || len(playop.Status.Conditions) == 0 {
		meta.SetStatusCondition(&playop.Status.Conditions, metav1.Condition{
			Type:    typeAvailablePlayOp,
			Status:  metav1.ConditionUnknown,
			Reason:  "Reconciling",
			Message: "Starting reconciliation",
		})
		if err = r.Status().Update(ctx, playop); err != nil {
			log.Error(err, "Failed to update PlayOp status")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the playop Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster
		if err := r.Get(ctx, req.NamespacedName, playop); err != nil {
			log.Error(err, "Failed to re-fetch playop")
			return ctrl.Result{}, err
		}
	}

	// Check if the pod(s) already exists, if not create them
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(playop.Namespace),
		client.MatchingLabels(labelsForPlayOpPods(playop.Name)),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods")
		return ctrl.Result{}, err
	}

	podsCount := len(podList.Items)
	size := playop.Spec.Size
	if podsCount < int(size) {
		// Create new pods to match the size value
		log.Info("Pods count less than expected", "Expected", size, "Found", podsCount)

		for i := podsCount; i < int(size); i++ {
			// Define new pod
			pod, err := r.podForPlayOp(playop, i)
			if err != nil {
				log.Error(err, "Failed to define new pod for PlayOp")
				meta.SetStatusCondition(&playop.Status.Conditions, metav1.Condition{
					Type:    typeAvailablePlayOp,
					Status:  metav1.ConditionFalse,
					Reason:  "Reconciling",
					Message: fmt.Sprintf("Failed to define new pod for the custom resource (%s): (%s)", playop.Name, err),
				})

				if err := r.Status().Update(ctx, playop); err != nil {
					log.Error(err, "Failed to update PlayOp status")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}

			// Create new pod
			err = r.Create(ctx, pod)
			if err != nil {
				log.Error(err, "Failed to create new pod for PlayOp", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
				return ctrl.Result{}, err
			}
		}

		// Pods created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if podsCount > int(size) {
		// Delete extra pods to match the size value
		log.Info("Pods count more than expected", "Expected", size, "Found", podsCount)

		for i := podsCount - 1; i >= int(size); i-- {
			pod := &corev1.Pod{}
			podToBeDeletedName := fmt.Sprintf("%v-pod-%v", playop.Name, i)
			// Verify pod exists
			err = r.Get(ctx, types.NamespacedName{Name: podToBeDeletedName, Namespace: playop.Namespace}, pod)
			if err != nil {
				log.Error(err, "Pod doesn't exist", "Pod.Name", podToBeDeletedName)
				meta.SetStatusCondition(&playop.Status.Conditions, metav1.Condition{
					Type:    typeAvailablePlayOp,
					Status:  metav1.ConditionFalse,
					Reason:  "Reconciling",
					Message: fmt.Sprintf("Failed to fetch the pod to be deleted for the custom resource (%s): (%s)", playop.Name, err),
				})

				if err = r.Status().Update(ctx, playop); err != nil {
					log.Error(err, "Failed to update PlayOp status")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}

			// Delete the pod
			err = r.Delete(ctx, pod, client.GracePeriodSeconds(0))
			if err != nil {
				log.Error(err, "Failed to delete the pod", "Pod.Name", podToBeDeletedName)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{Requeue: true}, nil
	}

	meta.SetStatusCondition(&playop.Status.Conditions, metav1.Condition{
		Type:    typeAvailablePlayOp,
		Status:  metav1.ConditionTrue,
		Reason:  "Reconciling",
		Message: fmt.Sprintf("Pods for custom resource (%s) created/deleted successfully", playop.Name),
	})

	podNames := getPodNames(podList.Items)
	playop.Status.Pods = podNames

	if err = r.Status().Update(ctx, playop); err != nil {
		log.Error(err, "Failed to update PlayOp status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// updatePodsCount updates the pods count in the namespace based on the countDiff,
// where if countDiff < 0 then existing pods count is more than expected value and
// if countDiff > 0 then exiting pods count is less than expected value
func (r *PlayOpReconciler) podForPlayOp(playop *cachev1alpha1.PlayOp, count int) (*corev1.Pod, error) {
	ls := labelsForPlayOpPods(playop.Name)
	podName := fmt.Sprintf("%v-pod-%v", playop.Name, count)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: playop.Namespace,
			Labels:    ls,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Image:           "nginx:latest",
				Name:            "nginx",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Ports: []corev1.ContainerPort{{
					ContainerPort: 80,
					Name:          "nginx",
				}},
			}},
		},
	}

	// Set the ownerRef for the Pod
	if err := ctrl.SetControllerReference(playop, pod, r.Scheme); err != nil {
		return nil, err
	}
	return pod, nil
}

// labelsForMemcached returns the labels for selecting the resources
func labelsForPlayOpPods(name string) map[string]string {
	imageTag := strings.Split("nginx:1.14.2", ":")[1]
	return map[string]string{"app.kubernetes.io/name": "PlayOp",
		"app.kubernetes.io/version":    imageTag,
		"app.kubernetes.io/part-of":    "playop-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}
}

// getPodNames returns the list of pods managed by the PlayOp CR
func getPodNames(podList []corev1.Pod) []string {
	podNames := []string{}
	for _, pod := range podList {
		podNames = append(podNames, pod.Name)
	}

	return podNames
}

// SetupWithManager sets up the controller with the Manager.
func (r *PlayOpReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.PlayOp{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
