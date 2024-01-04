# Copyright 2020 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# tu_int __umodti3(tu_int x, tu_int y)
# x is rsi:rdi, y is rcx:rdx, return result is rdx:rax.
.globl __umodti3
__umodti3:
	# specialized to u128 % u64, so verify that
	test %rcx,%rcx
	jne 1f

	# save divisor
	movq %rdx, %r8

	# reduce top 64 bits mod divisor
	movq %rsi, %rax
	xorl %edx, %edx
	divq %r8

	# reduce full 128-bit mod divisor
	# quotient fits in 64 bits because top 64 bits have been reduced < divisor.
	# (even though we only care about the remainder, divq also computes
	# the quotient, and it will trap if the quotient is too large.)
	movq %rdi, %rax
	divq %r8

	# expand remainder to 128 for return
	movq %rdx, %rax
	xorl %edx, %edx
	ret

1:
	# crash - only want 64-bit divisor
	xorl %ecx, %ecx
	movl %ecx, 0(%ecx)
	jmp 1b

.section .note.GNU-stack,"",@progbits
