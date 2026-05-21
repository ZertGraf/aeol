package simd

func hasAVX2() bool {
	if !hasCPUID() {
		return false
	}
	_, ebx, _, _ := cpuid(7, 0)
	return ebx&(1<<5) != 0
}

func hasCPUID() bool {
	return true
}

func cpuid(eaxArg, ecxArg uint32) (eax, ebx, ecx, edx uint32)
