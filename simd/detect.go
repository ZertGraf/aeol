package simd

import "runtime"

func detectBackend() Backend {
	switch runtime.GOARCH {
	case "amd64":
		if hasAVX2() {
			return &avx2Backend{}
		}
		return &scalarBackend{}
	case "arm64":
		return &scalarBackend{}
	default:
		return &scalarBackend{}
	}
}

func availableBackends() []BackendType {
	backends := []BackendType{Scalar}
	switch runtime.GOARCH {
	case "amd64":
		if hasAVX2() {
			backends = append(backends, AVX2)
		}
	}
	return backends
}
