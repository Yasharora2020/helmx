package helm

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Tests for client.go utility functions and pod status determination.
//
// These tests validate:
// - formatReady: Formats container ready counts as "X/Y" strings
// - formatInt: Converts integers to strings (restart counts)
// - determinePodStatus: Maps Kubernetes pod state to human-readable status
//
// The determinePodStatus function is critical for the Resources pane display,
// handling various pod states including init container failures, CrashLoopBackOff,
// ImagePullBackOff, and terminating pods.

// TestFormatReady verifies the "ready/total" string formatting used in pod status display.
func TestFormatReady(t *testing.T) {
	tests := []struct {
		ready    int
		total    int
		expected string
	}{
		{1, 1, "1/1"},
		{0, 1, "0/1"},
		{2, 3, "2/3"},
		{0, 0, "0/0"},
		{5, 5, "5/5"},
		{10, 12, "10/12"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatReady(tt.ready, tt.total)
			if result != tt.expected {
				t.Errorf("formatReady(%d, %d) = %s; want %s", tt.ready, tt.total, result, tt.expected)
			}
		})
	}
}

// TestFormatInt verifies integer-to-string conversion for restart counts.
func TestFormatInt(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{99, "99"},
		{100, "100"},
		{123, "123"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatInt(tt.input)
			if result != tt.expected {
				t.Errorf("formatInt(%d) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestDeterminePodStatus validates pod status detection for various Kubernetes pod states.
// This is critical for displaying accurate status in the Resources pane.
// Tests cover: Running, Pending, CrashLoopBackOff, ImagePullBackOff, Terminating,
// init container states (waiting, error), Succeeded, and Failed.
func TestDeterminePodStatus(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected string
	}{
		{
			name: "running pod",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{},
							},
						},
					},
				},
			},
			expected: "Running",
		},
		{
			name: "pending pod",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			expected: "Pending",
		},
		{
			name: "crashloopbackoff pod",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "CrashLoopBackOff",
								},
							},
						},
					},
				},
			},
			expected: "CrashLoopBackOff",
		},
		{
			name: "image pull backoff pod",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "ImagePullBackOff",
								},
							},
						},
					},
				},
			},
			expected: "ImagePullBackOff",
		},
		{
			name: "terminating pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &metav1.Time{},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			expected: "Terminating",
		},
		{
			name: "init container waiting",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					InitContainerStatuses: []corev1.ContainerStatus{
						{
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "PodInitializing",
								},
							},
						},
					},
				},
			},
			expected: "Init:PodInitializing",
		},
		{
			name: "init container error",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					InitContainerStatuses: []corev1.ContainerStatus{
						{
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									ExitCode: 1,
								},
							},
						},
					},
				},
			},
			expected: "Init:Error",
		},
		{
			name: "succeeded pod",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodSucceeded,
				},
			},
			expected: "Succeeded",
		},
		{
			name: "failed pod",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodFailed,
				},
			},
			expected: "Failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determinePodStatus(tt.pod)
			if result != tt.expected {
				t.Errorf("determinePodStatus() = %s; want %s", result, tt.expected)
			}
		})
	}
}
