package internal

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/syossan27/k8s-pending-resource-inspector/pkg/types"
	"k8s.io/apimachinery/pkg/api/resource"
)

type MockFetcher struct {
	mock.Mock
}

func (m *MockFetcher) FetchNodes(ctx context.Context) ([]types.NodeInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).([]types.NodeInfo), args.Error(1)
}

func (m *MockFetcher) FetchPendingPods(ctx context.Context, namespace string) ([]types.PodInfo, error) {
	args := m.Called(ctx, namespace)
	return args.Get(0).([]types.PodInfo), args.Error(1)
}

func TestNewAnalyzer(t *testing.T) {
	mockFetcher := &MockFetcher{}
	analyzer := NewAnalyzer(mockFetcher)

	assert.NotNil(t, analyzer)
	assert.Equal(t, mockFetcher, analyzer.fetcher)
}

func TestAnalyzePodSchedulability_Success(t *testing.T) {
	mockFetcher := &MockFetcher{}

	pods := []types.PodInfo{
		{
			Name:           "test-pod",
			Namespace:      "default",
			RequestsCPU:    resource.MustParse("100m"),
			RequestsMemory: resource.MustParse("128Mi"),
		},
	}

	nodes := []types.NodeInfo{
		{
			Name:              "node1",
			AllocatableCPU:    resource.MustParse("2"),
			AllocatableMemory: resource.MustParse("4Gi"),
		},
	}

	mockFetcher.On("FetchPendingPods", mock.Anything, "default").Return(pods, nil)
	mockFetcher.On("FetchNodes", mock.Anything).Return(nodes, nil)

	analyzer := NewAnalyzer(mockFetcher)
	ctx := context.Background()

	results, err := analyzer.AnalyzePodSchedulability(ctx, "default", false)

	require.NoError(t, err)
	assert.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, "test-pod", result.Pod.Name)
	assert.True(t, result.IsSchedulable)
	assert.Empty(t, result.Reason)
	assert.Empty(t, result.Suggestion)

	mockFetcher.AssertExpectations(t)
}

func TestAnalyzePodSchedulability_FetchPodsError(t *testing.T) {
	mockFetcher := &MockFetcher{}

	expectedError := errors.New("failed to fetch pods")
	mockFetcher.On("FetchPendingPods", mock.Anything, "default").Return([]types.PodInfo(nil), expectedError)

	analyzer := NewAnalyzer(mockFetcher)
	ctx := context.Background()

	results, err := analyzer.AnalyzePodSchedulability(ctx, "default", false)

	assert.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "failed to fetch pending pods")

	mockFetcher.AssertExpectations(t)
}

func TestAnalyzePodSchedulability_FetchNodesError(t *testing.T) {
	mockFetcher := &MockFetcher{}

	pods := []types.PodInfo{
		{
			Name:           "test-pod",
			Namespace:      "default",
			RequestsCPU:    resource.MustParse("100m"),
			RequestsMemory: resource.MustParse("128Mi"),
		},
	}

	expectedError := errors.New("failed to fetch nodes")
	mockFetcher.On("FetchPendingPods", mock.Anything, "default").Return(pods, nil)
	mockFetcher.On("FetchNodes", mock.Anything).Return([]types.NodeInfo(nil), expectedError)

	analyzer := NewAnalyzer(mockFetcher)
	ctx := context.Background()

	results, err := analyzer.AnalyzePodSchedulability(ctx, "default", false)

	assert.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "failed to fetch nodes")

	mockFetcher.AssertExpectations(t)
}

func TestAnalyzeSinglePod(t *testing.T) {
	tests := []struct {
		name           string
		pod            types.PodInfo
		nodes          []types.NodeInfo
		includeLimits  bool
		expectedResult types.AnalysisResult
	}{
		{
			name: "schedulable pod with requests",
			pod: types.PodInfo{
				Name:           "schedulable-pod",
				Namespace:      "default",
				RequestsCPU:    resource.MustParse("100m"),
				RequestsMemory: resource.MustParse("128Mi"),
			},
			nodes: []types.NodeInfo{
				{
					Name:              "node1",
					AllocatableCPU:    resource.MustParse("2"),
					AllocatableMemory: resource.MustParse("4Gi"),
				},
			},
			includeLimits: false,
			expectedResult: types.AnalysisResult{
				Pod: types.PodInfo{
					Name:           "schedulable-pod",
					Namespace:      "default",
					RequestsCPU:    resource.MustParse("100m"),
					RequestsMemory: resource.MustParse("128Mi"),
				},
				IsSchedulable:      true,
				Reason:             "",
				Suggestion:         "",
				MaxAvailableCPU:    resource.MustParse("2"),
				MaxAvailableMemory: resource.MustParse("4Gi"),
			},
		},
		{
			name: "unschedulable pod - CPU constraint",
			pod: types.PodInfo{
				Name:           "cpu-constrained-pod",
				Namespace:      "default",
				RequestsCPU:    resource.MustParse("3"),
				RequestsMemory: resource.MustParse("128Mi"),
			},
			nodes: []types.NodeInfo{
				{
					Name:              "node1",
					AllocatableCPU:    resource.MustParse("2"),
					AllocatableMemory: resource.MustParse("4Gi"),
				},
			},
			includeLimits: false,
			expectedResult: types.AnalysisResult{
				Pod: types.PodInfo{
					Name:           "cpu-constrained-pod",
					Namespace:      "default",
					RequestsCPU:    resource.MustParse("3"),
					RequestsMemory: resource.MustParse("128Mi"),
				},
				IsSchedulable:      false,
				Reason:             "requests.cpu = 3 exceeds all node allocatable.cpu (max: 2)",
				Suggestion:         "Lower requests.cpu to <= 2 or add higher-CPU node",
				MaxAvailableCPU:    resource.MustParse("2"),
				MaxAvailableMemory: resource.MustParse("4Gi"),
			},
		},
		{
			name: "unschedulable pod - memory constraint",
			pod: types.PodInfo{
				Name:           "memory-constrained-pod",
				Namespace:      "default",
				RequestsCPU:    resource.MustParse("100m"),
				RequestsMemory: resource.MustParse("8Gi"),
			},
			nodes: []types.NodeInfo{
				{
					Name:              "node1",
					AllocatableCPU:    resource.MustParse("2"),
					AllocatableMemory: resource.MustParse("4Gi"),
				},
			},
			includeLimits: false,
			expectedResult: types.AnalysisResult{
				Pod: types.PodInfo{
					Name:           "memory-constrained-pod",
					Namespace:      "default",
					RequestsCPU:    resource.MustParse("100m"),
					RequestsMemory: resource.MustParse("8Gi"),
				},
				IsSchedulable:      false,
				Reason:             "requests.memory = 8Gi exceeds all node allocatable.memory (max: 4Gi)",
				Suggestion:         "Lower requests.memory to <= 4Gi or add higher-memory node",
				MaxAvailableCPU:    resource.MustParse("2"),
				MaxAvailableMemory: resource.MustParse("4Gi"),
			},
		},
		{
			name: "unschedulable pod - both CPU and memory constraints",
			pod: types.PodInfo{
				Name:           "both-constrained-pod",
				Namespace:      "default",
				RequestsCPU:    resource.MustParse("3"),
				RequestsMemory: resource.MustParse("8Gi"),
			},
			nodes: []types.NodeInfo{
				{
					Name:              "node1",
					AllocatableCPU:    resource.MustParse("2"),
					AllocatableMemory: resource.MustParse("4Gi"),
				},
			},
			includeLimits: false,
			expectedResult: types.AnalysisResult{
				Pod: types.PodInfo{
					Name:           "both-constrained-pod",
					Namespace:      "default",
					RequestsCPU:    resource.MustParse("3"),
					RequestsMemory: resource.MustParse("8Gi"),
				},
				IsSchedulable:      false,
				Reason:             "requests.cpu = 3 and requests.memory = 8Gi exceed all node allocatable resources (max CPU: 2, max memory: 4Gi)",
				Suggestion:         "Lower requests.cpu to <= 2 and requests.memory to <= 4Gi, or add nodes with higher capacity",
				MaxAvailableCPU:    resource.MustParse("2"),
				MaxAvailableMemory: resource.MustParse("4Gi"),
			},
		},
		{
			name: "schedulable pod with limits enabled",
			pod: types.PodInfo{
				Name:           "limits-pod",
				Namespace:      "default",
				RequestsCPU:    resource.MustParse("100m"),
				RequestsMemory: resource.MustParse("128Mi"),
				LimitsCPU:      resource.MustParse("500m"),
				LimitsMemory:   resource.MustParse("512Mi"),
			},
			nodes: []types.NodeInfo{
				{
					Name:              "node1",
					AllocatableCPU:    resource.MustParse("2"),
					AllocatableMemory: resource.MustParse("4Gi"),
				},
			},
			includeLimits: true,
			expectedResult: types.AnalysisResult{
				Pod: types.PodInfo{
					Name:           "limits-pod",
					Namespace:      "default",
					RequestsCPU:    resource.MustParse("100m"),
					RequestsMemory: resource.MustParse("128Mi"),
					LimitsCPU:      resource.MustParse("500m"),
					LimitsMemory:   resource.MustParse("512Mi"),
				},
				IsSchedulable:      true,
				Reason:             "",
				Suggestion:         "",
				MaxAvailableCPU:    resource.MustParse("2"),
				MaxAvailableMemory: resource.MustParse("4Gi"),
			},
		},
		{
			name: "unschedulable pod with limits - CPU constraint",
			pod: types.PodInfo{
				Name:           "limits-cpu-constrained-pod",
				Namespace:      "default",
				RequestsCPU:    resource.MustParse("100m"),
				RequestsMemory: resource.MustParse("128Mi"),
				LimitsCPU:      resource.MustParse("3"),
				LimitsMemory:   resource.MustParse("512Mi"),
			},
			nodes: []types.NodeInfo{
				{
					Name:              "node1",
					AllocatableCPU:    resource.MustParse("2"),
					AllocatableMemory: resource.MustParse("4Gi"),
				},
			},
			includeLimits: true,
			expectedResult: types.AnalysisResult{
				Pod: types.PodInfo{
					Name:           "limits-cpu-constrained-pod",
					Namespace:      "default",
					RequestsCPU:    resource.MustParse("100m"),
					RequestsMemory: resource.MustParse("128Mi"),
					LimitsCPU:      resource.MustParse("3"),
					LimitsMemory:   resource.MustParse("512Mi"),
				},
				IsSchedulable:      false,
				Reason:             "limits.cpu = 3 exceeds all node allocatable.cpu (max: 2)",
				Suggestion:         "Lower limits.cpu to <= 2 or add higher-CPU node",
				MaxAvailableCPU:    resource.MustParse("2"),
				MaxAvailableMemory: resource.MustParse("4Gi"),
			},
		},
		{
			name: "pod with partial limits - only CPU limit set",
			pod: types.PodInfo{
				Name:           "partial-limits-pod",
				Namespace:      "default",
				RequestsCPU:    resource.MustParse("100m"),
				RequestsMemory: resource.MustParse("128Mi"),
				LimitsCPU:      resource.MustParse("500m"),
			},
			nodes: []types.NodeInfo{
				{
					Name:              "node1",
					AllocatableCPU:    resource.MustParse("2"),
					AllocatableMemory: resource.MustParse("4Gi"),
				},
			},
			includeLimits: true,
			expectedResult: types.AnalysisResult{
				Pod: types.PodInfo{
					Name:           "partial-limits-pod",
					Namespace:      "default",
					RequestsCPU:    resource.MustParse("100m"),
					RequestsMemory: resource.MustParse("128Mi"),
					LimitsCPU:      resource.MustParse("500m"),
				},
				IsSchedulable:      true,
				Reason:             "",
				Suggestion:         "",
				MaxAvailableCPU:    resource.MustParse("2"),
				MaxAvailableMemory: resource.MustParse("4Gi"),
			},
		},
	}

	analyzer := &Analyzer{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.analyzeSinglePod(tt.pod, tt.nodes, tt.includeLimits)

			assert.Equal(t, tt.expectedResult.Pod.Name, result.Pod.Name)
			assert.Equal(t, tt.expectedResult.Pod.Namespace, result.Pod.Namespace)
			assert.Equal(t, tt.expectedResult.IsSchedulable, result.IsSchedulable)
			assert.Equal(t, tt.expectedResult.Reason, result.Reason)
			assert.Equal(t, tt.expectedResult.Suggestion, result.Suggestion)
			assert.True(t, tt.expectedResult.MaxAvailableCPU.Equal(result.MaxAvailableCPU))
			assert.True(t, tt.expectedResult.MaxAvailableMemory.Equal(result.MaxAvailableMemory))
		})
	}
}

func TestFindMaxAvailableResources(t *testing.T) {
	tests := []struct {
		name              string
		nodes             []types.NodeInfo
		expectedMaxCPU    resource.Quantity
		expectedMaxMemory resource.Quantity
	}{
		{
			name: "single node",
			nodes: []types.NodeInfo{
				{
					Name:              "node1",
					AllocatableCPU:    resource.MustParse("2"),
					AllocatableMemory: resource.MustParse("4Gi"),
				},
			},
			expectedMaxCPU:    resource.MustParse("2"),
			expectedMaxMemory: resource.MustParse("4Gi"),
		},
		{
			name: "multiple nodes - different max resources",
			nodes: []types.NodeInfo{
				{
					Name:              "node1",
					AllocatableCPU:    resource.MustParse("2"),
					AllocatableMemory: resource.MustParse("8Gi"),
				},
				{
					Name:              "node2",
					AllocatableCPU:    resource.MustParse("4"),
					AllocatableMemory: resource.MustParse("4Gi"),
				},
				{
					Name:              "node3",
					AllocatableCPU:    resource.MustParse("1"),
					AllocatableMemory: resource.MustParse("16Gi"),
				},
			},
			expectedMaxCPU:    resource.MustParse("4"),
			expectedMaxMemory: resource.MustParse("16Gi"),
		},
		{
			name:              "empty nodes",
			nodes:             []types.NodeInfo{},
			expectedMaxCPU:    resource.Quantity{},
			expectedMaxMemory: resource.Quantity{},
		},
		{
			name: "nodes with zero resources",
			nodes: []types.NodeInfo{
				{
					Name:              "node1",
					AllocatableCPU:    resource.MustParse("0"),
					AllocatableMemory: resource.MustParse("0"),
				},
				{
					Name:              "node2",
					AllocatableCPU:    resource.MustParse("1"),
					AllocatableMemory: resource.MustParse("1Gi"),
				},
			},
			expectedMaxCPU:    resource.MustParse("1"),
			expectedMaxMemory: resource.MustParse("1Gi"),
		},
	}

	analyzer := &Analyzer{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxCPU, maxMemory := analyzer.findMaxAvailableResources(tt.nodes)

			assert.True(t, tt.expectedMaxCPU.Equal(maxCPU), "Expected CPU %s, got %s", tt.expectedMaxCPU.String(), maxCPU.String())
			assert.True(t, tt.expectedMaxMemory.Equal(maxMemory), "Expected Memory %s, got %s", tt.expectedMaxMemory.String(), maxMemory.String())
		})
	}
}

func TestEvaluateResourceConstraints(t *testing.T) {
	analyzer := &Analyzer{}
	ctx := context.Background()

	err := analyzer.EvaluateResourceConstraints(ctx)

	assert.NoError(t, err)
}
