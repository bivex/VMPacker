package main

import (
	"fmt"
	"github.com/vmpacker/pkg/arch/x86_64"
	"github.com/vmpacker/pkg/vm"
	"golang.org/x/arch/x86/x86asm"
)

// Define the C sizes
func cSize(opId byte) int {
	switch opId {
	case vm.OpIdNop, vm.OpIdHalt, vm.OpIdSLoadSlide, vm.OpIdSDup, vm.OpIdSSwap, vm.OpIdSDrop,
		vm.OpIdSAdd, vm.OpIdSSub, vm.OpIdSMul, vm.OpIdSXor, vm.OpIdSAnd, vm.OpIdSOr,
		vm.OpIdSShl, vm.OpIdSShr, vm.OpIdSAsr, vm.OpIdSRor, vm.OpIdSUmulh, vm.OpIdSSmulh,
		vm.OpIdSUdiv, vm.OpIdSSdiv, vm.OpIdSAdc, vm.OpIdSSbc, vm.OpIdSNot, vm.OpIdSNeg,
		vm.OpIdSClz, vm.OpIdSCls, vm.OpIdSRbit, vm.OpIdSRev, vm.OpIdSRev16, vm.OpIdSRev32,
		vm.OpIdSTrunc32, vm.OpIdSSext32, vm.OpIdSCmp, vm.OpIdSLd8, vm.OpIdSLd16, vm.OpIdSLd32,
		vm.OpIdSLd64, vm.OpIdSSt8, vm.OpIdSSt16, vm.OpIdSSt32, vm.OpIdSSt64, vm.OpIdSDecryptStr:
		return 1
	case vm.OpIdPush, vm.OpIdPop, vm.OpIdCallReg, vm.OpIdBrReg, vm.OpIdRet,
		vm.OpIdSVload, vm.OpIdSVstore, vm.OpIdSVloadV, vm.OpIdSVstoreV:
		return 2
	case vm.OpIdMovReg, vm.OpIdCmp, vm.OpIdNot, vm.OpIdVld16, vm.OpIdVst16,
		vm.OpIdSvc, vm.OpIdClz, vm.OpIdCls, vm.OpIdRbit, vm.OpIdRev, vm.OpIdRev16,
		vm.OpIdRev32, vm.OpIdSVLd, vm.OpIdSVSt:
		return 3
	case vm.OpIdAdd, vm.OpIdSub, vm.OpIdMul, vm.OpIdXor, vm.OpIdAnd, vm.OpIdOr,
		vm.OpIdShl, vm.OpIdShr, vm.OpIdAsr, vm.OpIdRor, vm.OpIdUmulh, vm.OpIdUdiv,
		vm.OpIdSdiv, vm.OpIdMrs, vm.OpIdSmulh, vm.OpIdAdc, vm.OpIdSbc, vm.OpIdSFMov,
		vm.OpIdSFCmp, vm.OpIdSFNeg, vm.OpIdSFAbs, vm.OpIdSFSqrt, vm.OpIdSFCvtIF,
		vm.OpIdSFCvtFI, vm.OpIdSFMovRV, vm.OpIdSFMovVR, vm.OpIdSFCvt:
		return 4
	case vm.OpIdJmp, vm.OpIdJe, vm.OpIdJne, vm.OpIdJl, vm.OpIdJge, vm.OpIdJgt,
		vm.OpIdJle, vm.OpIdJb, vm.OpIdJae, vm.OpIdJbe, vm.OpIdJa, vm.OpIdJvs,
		vm.OpIdJvc, vm.OpIdSPushImm32, vm.OpIdSFAdd, vm.OpIdSFSub, vm.OpIdSFMul,
		vm.OpIdSFDiv, vm.OpIdSFMax, vm.OpIdSFMin:
		return 5
	case vm.OpIdMovImm32, vm.OpIdCmpImm, vm.OpIdCcmpReg, vm.OpIdCcmpImm,
		vm.OpIdCcmnReg, vm.OpIdCcmnImm:
		return 6
	case vm.OpIdAddImm, vm.OpIdSubImm, vm.OpIdXorImm, vm.OpIdAndImm, vm.OpIdOrImm,
		vm.OpIdMulImm, vm.OpIdShlImm, vm.OpIdShrImm, vm.OpIdAsrImm, vm.OpIdTbz, vm.OpIdTbnz:
		return 7
	case vm.OpIdCallNative, vm.OpIdSPushImm64, vm.OpIdSnprintf:
		return 9
	case vm.OpIdMovImm:
		return 10
	case vm.OpIdSNativeExec:
		return 4
	}
	return 1
}

func main() {
	vm.GenerateDynamicISA()
	
	// Create instructions for check1
	// 55 push rbp
	// 48 89 e5 mov rsp, rbp
	// 89 7d fc mov edi, -0x4(rbp)
	// 90 nop
	// 8b 45 fc mov -0x4(rbp), eax
	// 83 c0 0b add 0xb, eax
	// 5d pop rbp
	// c3 ret
	
	raws := [][]byte{
		{0x55},
		{0x48, 0x89, 0xe5},
		{0x89, 0x7d, 0xfc},
		{0x90},
		{0x8b, 0x45, 0xfc},
		{0x83, 0xc0, 0x0b},
		{0x5d},
		{0xc3},
	}
	
	t := x86_64.NewTranslator(0x400000, 100, nil)
	
	var insts []vm.Instruction
	off := 0
	for _, raw := range raws {
		insts = append(insts, vm.Instruction{Offset: off, RawBytes: raw})
		off += len(raw)
	}
	
	res, err := t.Translate(insts)
	if err != nil {
		panic(err)
	}
	
	fmt.Printf("Translation successful, length: %d\n", res.CodeLen)
	
	// Encrypt
	ocKey := uint32(0xdeadbeef)
	bc := make([]byte, res.CodeLen)
	copy(bc, res.Bytecode)
	
	// encryptOpcodes logic
	pc := 0
	for pc < res.CodeLen {
		opcode := bc[pc]
		info := vm.OpTable()[opcode]
		
		mask := byte(ocKey ^ (uint32(pc) * 0x9E3779B9))
		bc[pc] ^= mask
		pc += info.Size
	}
	
	// Now decrypt like C interpreter
	pc = 0
	for pc < res.CodeLen {
		raw := bc[pc]
		mask := byte(ocKey ^ (uint32(pc) * 0x9E3779B9))
		dec := raw ^ mask
		
		// Find logical opcode
		logical := vm.InverseOpMap[dec]
		info := vm.OpTable()[dec]
		cSz := cSize(logical)
		
		if info.Size != cSz {
			fmt.Printf("MISMATCH at PC=%d: %s (logical %d) GoSize=%d CSize=%d\n", pc, info.Name, logical, info.Size, cSz)
		} else {
			fmt.Printf("OK at PC=%d: %s (logical %d) size=%d\n", pc, info.Name, logical, cSz)
		}
		
		pc += cSz
	}
	
	fmt.Printf("Final PC: %d (expected %d)\n", pc, res.CodeLen)
}
