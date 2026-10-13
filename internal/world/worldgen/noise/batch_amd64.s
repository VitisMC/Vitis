//go:build amd64 && !purego

#include "textflag.h"
#include "go_asm.h"

// Four doubles are evaluated independently, with no fused instructions.
DATA ·batchOne+0(SB)/8, $0x3ff0000000000000
GLOBL ·batchOne(SB), RODATA|NOPTR, $8

DATA ·batchSmooth+0(SB)/8, $0x4018000000000000
DATA ·batchSmooth+8(SB)/8, $0x402e000000000000
DATA ·batchSmooth+16(SB)/8, $0x4024000000000000
GLOBL ·batchSmooth(SB), RODATA|NOPTR, $24

TEXT ·noiseCPUID(SB), NOSPLIT, $0-24
	MOVL leaf+0(FP), AX
	MOVL subleaf+4(FP), CX
	CPUID
	MOVL AX, a+8(FP)
	MOVL BX, b+12(FP)
	MOVL CX, c+16(FP)
	MOVL DX, d+20(FP)
	RET

TEXT ·noiseXGETBV(SB), NOSPLIT, $0-8
	XORL CX, CX
	XGETBV
	MOVL AX, a+0(FP)
	MOVL DX, d+4(FP)
	RET

// GRAD selects the two nonzero signed axes from each gradient hash.
#define GRAD(INDEX, OX, OY, OZ, OUTPUT) \
 VPMOVZXBQ improvedBatchInput_gradients+INDEX(AX), Y0; \
 VBROADCASTSD improvedBatchInput_x(AX), Y5; \
 VSUBPD OX, Y5, Y5; \
 VMOVUPD improvedBatchInput_y(AX), Y6; \
 VSUBPD OY, Y6, Y6; \
 VBROADCASTSD improvedBatchInput_z(AX), Y7; \
 VSUBPD OZ, Y7, Y7; \
 VPCMPGTQ Y0, Y10, Y1; \
 VBLENDVPD Y1, Y5, Y6, Y2; \
 VPSLLQ $63, Y0, Y14; \
 VXORPD Y14, Y2, Y2; \
 VPCMPEQQ Y12, Y0, Y15; \
 VPCMPEQQ Y13, Y0, Y4; \
 VPOR Y4, Y15, Y15; \
 VBLENDVPD Y15, Y5, Y7, Y3; \
 VPCMPGTQ Y0, Y11, Y1; \
 VBLENDVPD Y1, Y6, Y3, Y3; \
 VPSRLQ $1, Y0, Y14; \
 VPSLLQ $63, Y14, Y14; \
 VXORPD Y14, Y3, Y3; \
 VADDPD Y3, Y2, Y2; \
 VMOVUPD Y2, OUTPUT(SP)

DATA ·gradientLimits+0(SB)/8, $8
DATA ·gradientLimits+8(SB)/8, $4
DATA ·gradientLimits+16(SB)/8, $12
DATA ·gradientLimits+24(SB)/8, $14
GLOBL ·gradientLimits(SB), RODATA|NOPTR, $32

TEXT ·improvedBatchAVX2(SB), NOSPLIT, $256-16
	MOVQ input+0(FP), AX
	VXORPD Y9, Y9, Y9
	VBROADCASTSD ·batchOne(SB), Y8
	VBROADCASTSD ·gradientLimits+0(SB), Y10
	VBROADCASTSD ·gradientLimits+8(SB), Y11
	VBROADCASTSD ·gradientLimits+16(SB), Y12
	VBROADCASTSD ·gradientLimits+24(SB), Y13
	GRAD(0, Y9, Y9, Y9, 0)
	GRAD(4, Y8, Y9, Y9, 32)
	GRAD(8, Y9, Y8, Y9, 64)
	GRAD(12, Y8, Y8, Y9, 96)
	GRAD(16, Y9, Y9, Y8, 128)
	GRAD(20, Y8, Y9, Y8, 160)
	GRAD(24, Y9, Y8, Y8, 192)
	GRAD(28, Y8, Y8, Y8, 224)
	VBROADCASTSD improvedBatchInput_smoothX(AX), Y8
	VMOVUPD 0(SP), Y0
	VMOVUPD 32(SP), Y1
	VSUBPD Y0, Y1, Y1
	VMULPD Y8, Y1, Y1
	VADDPD Y1, Y0, Y0
	VMOVUPD 64(SP), Y2
	VMOVUPD 96(SP), Y3
	VSUBPD Y2, Y3, Y3
	VMULPD Y8, Y3, Y3
	VADDPD Y3, Y2, Y2
	VMOVUPD 128(SP), Y4
	VMOVUPD 160(SP), Y5
	VSUBPD Y4, Y5, Y5
	VMULPD Y8, Y5, Y5
	VADDPD Y5, Y4, Y4
	VMOVUPD 192(SP), Y6
	VMOVUPD 224(SP), Y7
	VSUBPD Y6, Y7, Y7
	VMULPD Y8, Y7, Y7
	VADDPD Y7, Y6, Y6
	VMOVUPD improvedBatchInput_fractionY(AX), Y9
	// Preserve value*value*value*(value*(value*6-15)+10) in four lanes.
	VMULPD Y9, Y9, Y10
	VMULPD Y9, Y10, Y10
	VBROADCASTSD ·batchSmooth+0(SB), Y11
	VMULPD Y11, Y9, Y11
	VBROADCASTSD ·batchSmooth+8(SB), Y12
	VSUBPD Y12, Y11, Y11
	VMULPD Y11, Y9, Y11
	VBROADCASTSD ·batchSmooth+16(SB), Y12
	VADDPD Y12, Y11, Y11
	VMULPD Y11, Y10, Y9
	VSUBPD Y0, Y2, Y2
	VMULPD Y9, Y2, Y2
	VADDPD Y2, Y0, Y0
	VSUBPD Y4, Y6, Y6
	VMULPD Y9, Y6, Y6
	VADDPD Y6, Y4, Y4
	VBROADCASTSD improvedBatchInput_smoothZ(AX), Y8
	VSUBPD Y0, Y4, Y4
	VMULPD Y8, Y4, Y4
	VADDPD Y4, Y0, Y0
	MOVQ result+8(FP), AX
	VMOVUPD Y0, (AX)
	VZEROUPPER
	RET
