# Integration Tests

This directory contains comprehensive integration tests for the k8s-pending-resource-inspector CLI tool.

## Overview

The integration tests validate end-to-end functionality by:
- Setting up mock Kubernetes clusters with predefined configurations
- Testing complete workflows from CLI input to output
- Validating all output formats (human, JSON, YAML)
- Testing error conditions and edge cases
- Verifying performance with large cluster configurations

## Test Structure

### Files

- `integration_test.go` - Main integration test suite covering core functionality
- `cli_integration_test.go` - CLI-specific integration tests and real-world scenarios
- `README.md` - This documentation file

### Test Categories

1. **End-to-End Workflow Tests**
   - Schedulable pods scenarios
   - Unschedulable pods scenarios  
   - Mixed scenarios
   - Namespace-specific analysis
   - Resource limits analysis

2. **Output Format Tests**
   - Human-readable output validation
   - JSON structure validation
   - YAML structure validation

3. **Error Condition Tests**
   - Empty cluster scenarios
   - Unsupported output formats
   - API failure simulation

4. **CLI Flag Combination Tests**
   - Different namespace configurations
   - Include/exclude limits flag
   - Various output format combinations

5. **Performance Tests**
   - Large cluster scenarios (100+ nodes)
   - High pod count scenarios
   - Performance benchmarking

6. **Real-World Scenarios**
   - Production-like cluster configurations
   - Resource exhaustion scenarios
   - Node taints and tolerations

## Running Integration Tests

### Prerequisites

- Go 1.21 or later
- All project dependencies installed (`go mod download`)

### Commands

```bash
# Run all integration tests
go test -v -tags=integration ./test/integration/...

# Run specific integration test
go test -v -tags=integration ./test/integration/ -run TestIntegrationSuite

# Run with race detection
go test -race -v -tags=integration ./test/integration/...

# Run with coverage
go test -v -tags=integration -coverprofile=integration-coverage.out ./test/integration/...
```

### CI Integration

Integration tests are automatically executed in the CI pipeline after unit tests pass. See `.github/workflows/ci.yml` for the complete CI configuration.

## Test Data and Fixtures

The integration tests use programmatically generated Kubernetes resources:

- **Nodes**: Various configurations (CPU/memory optimized, balanced, tainted)
- **Pods**: Different resource requirements (schedulable, unschedulable, mixed)
- **Clusters**: Small, medium, large, and heterogeneous configurations

## Mock Infrastructure

Tests use `fake.NewSimpleClientset()` from the Kubernetes client-go library to create mock Kubernetes API servers. This provides:

- Realistic API behavior without requiring a real cluster
- Deterministic test results
- Fast test execution
- Complete control over cluster state

## Coverage

Integration tests cover all acceptance criteria from GitHub issue #17:

- ✅ Mock Kubernetes API server for testing
- ✅ Test complete workflows from CLI input to output  
- ✅ Cover scenarios: schedulable pods, unschedulable pods, no pods
- ✅ Test all output formats (human-readable, JSON, YAML)
- ✅ Test error conditions and different cluster configurations
- ✅ Performance testing with large clusters (100+ nodes)
- ✅ Slack notification integration testing
- ✅ CLI flag combination testing
- ✅ Real-world production scenarios

## Architecture

The integration tests follow a layered approach:

1. **Mock Infrastructure Layer**: Uses `fake.NewSimpleClientset()` to simulate Kubernetes API
2. **Test Data Layer**: Programmatically generated nodes and pods with various configurations
3. **Workflow Testing Layer**: End-to-end testing of the complete application pipeline
4. **Validation Layer**: Output format validation and result verification

## Maintenance

When adding new features to the main application:

1. Add corresponding integration test scenarios
2. Update test data fixtures if needed
3. Verify all existing integration tests still pass
4. Update this documentation if test structure changes

## Troubleshooting

Common issues and solutions:

- **Tests fail with "no test files"**: Ensure you're using the `-tags=integration` flag
- **Mock data issues**: Check that node and pod resource specifications are valid
- **CI failures**: Verify that the integration test step is properly configured in `.github/workflows/ci.yml`
