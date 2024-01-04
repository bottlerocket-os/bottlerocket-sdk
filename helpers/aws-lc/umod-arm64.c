// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

typedef unsigned int u128 __attribute__((mode(TI)));

static u128 div(u128 x, u128 y, u128 *rp) {
	int n = 0;
	while((y>>(128-1)) != 1 && y < x) {
		y<<=1;
		n++;
	}
	u128 q = 0;
	for(;; n--, y>>=1, q<<=1) {
		if(x>=y) {
			x -= y;
			q |= 1;
		}
		if(n == 0)
			break;
	}
	if(rp)
		*rp = x;
	return q;
}

u128 __umodti3(u128 x, u128 y) {
	u128 r;
	div(x, y, &r);
	return r;
}

u128 __udivti3(u128 x, u128 y) {
	return div(x, y, 0);
}
