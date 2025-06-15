// NOTE: This code is basically a translation of firedancer/src/ballet/base58 without the support for AVX instrcutions.

package base58

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	BASE58_ALPHABET string = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	BASE58_INVALID_CHAR           = 255
	BASE58_INVERSE_TABLE_OFFSET   = '1'
	BASE58_INVERSE_TABLE_SENTINEL = 1 + 'z' - BASE58_INVERSE_TABLE_OFFSET

	BASE58_ENCODED_32_LEN = 44
	BASE58_ENCODED_64_LEN = 88
	RAWBASE58_32_SZ       = 45
	RAWBASE58_64_SZ       = 90
	INTERMEDIATE_32_SZ    = 9
	INTERMEDIATE_64_SZ    = 18
	BINARY_32_SZ          = 8
	BINARY_64_SZ          = 16
)

var (
	BASE58_INVERSE = []byte{
		0, 1, 2, 3, 4, 5, 6, 7, 8, 255,
		255, 255, 255, 255, 255, 255, 9, 10, 11, 12,
		13, 14, 15, 16, 255, 17, 18, 19, 20, 21,
		255, 22, 23, 24, 25, 26, 27, 28, 29, 30,
		31, 32, 255, 255, 255, 255, 255, 255, 33, 34,
		35, 36, 37, 38, 39, 40, 41, 42, 43, 255,
		44, 45, 46, 47, 48, 49, 50, 51, 52, 53,
		54, 55, 56, 57, 255,
	}
)

func Encode32(data [32]byte) string {
	inLeadingZeros := 0
	for inLeadingZeros = 0; inLeadingZeros < 32; inLeadingZeros++ {
		if data[inLeadingZeros] != 0 {
			break
		}
	}

	/* X = sum_i bytes[i] * 2^(8*(BYTE_CNT-1-i)) */
	/* Convert N to 32-bit limbs:
	   X = sum_i bin[i] * 2^(32*(BINARY_SZ-1-i)) */
	var bin [8]uint32
	for i := 0; i < 8; i++ {
		bin[i] = binary.BigEndian.Uint32(data[i*4:])
	}

	var r1Div uint64 = 656356768
	/* Convert to the intermediate format:
	     X = sum_i intermediate[i] * 58^(5*(INTERMEDIATE_SZ-1-i))
	   Initially, we don't require intermediate[i] < 58^5, but we do want
	   to make sure the sums don't overflow. */
	var intermediate [9]uint64

	/* The worst case is if binary[7] is (2^32)-1. In that case
	   intermediate[8] will be just over 2^63, which is fine. */
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			intermediate[j+1] += uint64(bin[i]) * uint64(ENC_TABLE_32[i][j])
		}
	}

	/* Now we make sure each term is less than 58^5. Again, we have to be
	   a bit careful of overflow.

	   For N==32, in the worst case, as before, intermediate[8] will be
	   just over 2^63 and intermediate[7] will be just over 2^62.6.  In
	   the first step, we'll add floor(intermediate[8]/58^5) to
	   intermediate[7].  58^5 is pretty big though, so intermediate[7]
	   barely budges, and this is still fine.

	   For N==64, in the worst case, the biggest entry in intermediate at
	   this point is 2^63.87, and in the worst case, we add (2^64-1)/58^5,
	   which is still about 2^63.87. */
	for i := 8; i > 0; i-- {
		intermediate[i-1] += intermediate[i] / r1Div
		intermediate[i] %= r1Div
	}

	/* Convert intermediate form to base 58.  This form of conversion
	   exposes tons of ILP, but it's more than the CPU can take advantage
	   of.
	     X = sum_i raw_base58[i] * 58^(RAW58_SZ-1-i) */
	var rawBase58 [45]byte
	for i := 0; i < 9; i++ {
		/* We know intermediate[ i ] < 58^5 < 2^32 for all i, so casting to
		   a uint is safe.  GCC doesn't seem to be able to realize this, so
		   when it converts ulong/ulong to a magic multiplication, it
		   generates the single-op 64b x 64b -> 128b mul instruction.  This
		   hurts the CPU's ability to take advantage of the ILP here. */
		v := uint(intermediate[i])
		rawBase58[5*i+4] = byte((v / 1) % 58)
		rawBase58[5*i+3] = byte((v / 58) % 58)
		rawBase58[5*i+2] = byte((v / 3364) % 58)
		rawBase58[5*i+1] = byte((v / 195112) % 58)
		rawBase58[5*i+0] = byte(v / 11316496)
	}

	/* Finally, actually convert to the string.  We have to ignore all the
	   leading zeros in raw_base58 and instead insert in_leading_0s
	   leading '1' characters.  We can show that raw_base58 actually has
	   at least in_leading_0s, so we'll do this by skipping the first few
	   leading zeros in raw_base58. */
	rawLeadingZeros := 0
	for ; rawLeadingZeros < 45; rawLeadingZeros++ {
		if rawBase58[rawLeadingZeros] != 0 {
			break
		}
	}

	/* It's not immediately obvious that raw_leading_0s >= in_leading_0s,
	   but it's true.  In base b, X has floor(log_b X)+1 digits.  That
	   means in_leading_0s = N-1-floor(log_256 X) and raw_leading_0s =
	   RAW58_SZ-1-floor(log_58 X).  Let X<256^N be given and consider:

	   raw_leading_0s - in_leading_0s =
	     =  RAW58_SZ-N + floor( log_256 X ) - floor( log_58 X )
	     >= RAW58_SZ-N - 1 + ( log_256 X - log_58 X ) .

	   log_256 X - log_58 X is monotonically decreasing for X>0, so it
	   achieves it minimum at the maximum possible value for X, i.e.
	   256^N-1.
	     >= RAW58_SZ-N-1 + log_256(256^N-1) - log_58(256^N-1)

	   When N==32, RAW58_SZ is 45, so this gives skip >= 0.29
	   When N==64, RAW58_SZ is 90, so this gives skip >= 1.59.

	   Regardless, raw_leading_0s - in_leading_0s >= 0. */
	var out [44]byte
	skip := rawLeadingZeros - inLeadingZeros
	for i := 0; i < 45-skip; i++ {
		out[i] = BASE58_ALPHABET[rawBase58[skip+i]]
	}

	return string(out[:45-skip])
}

func Encode64(data [64]byte) string {
	inLeadingZeros := 0
	for inLeadingZeros = 0; inLeadingZeros < 64; inLeadingZeros++ {
		if data[inLeadingZeros] != 0 {
			break
		}
	}

	/* X = sum_i bytes[i] * 2^(8*(BYTE_CNT-1-i)) */
	/* Convert N to 32-bit limbs:
	   X = sum_i bin[i] * 2^(32*(BINARY_SZ-1-i)) */
	var bin [16]uint32
	for i := 0; i < 16; i++ {
		bin[i] = binary.BigEndian.Uint32(data[i*4:])
	}

	var r1Div uint64 = 656356768
	/* Convert to the intermediate format:
	     X = sum_i intermediate[i] * 58^(5*(INTERMEDIATE_SZ-1-i))
	   Initially, we don't require intermediate[i] < 58^5, but we do want
	   to make sure the sums don't overflow. */
	var intermediate [18]uint64

	/* If we do it the same way as the 32B conversion, intermediate[16]
	   can overflow when the input is sufficiently large.  We'll do a
	   mini-reduction after the first 8 steps.  After the first 8 terms,
	   the largest intermediate[16] can be is 2^63.87.  Then, after
	   reduction it'll be at most 58^5, and after adding the last terms,
	   it won't exceed 2^63.1.  We do need to be cautious that the
	   mini-reduction doesn't cause overflow in intermediate[15] though.
	   Pre-mini-reduction, it's at most 2^63.05.  The mini-reduction adds
	   at most 2^64/58^5, which is negligible.  With the final terms, it
	   won't exceed 2^63.69, which is fine. Other terms are less than
	   2^63.76, so no problems there. */

	for i := 0; i < 8; i++ {
		for j := 0; j < 17; j++ {
			intermediate[j+1] += uint64(bin[i]) * uint64(ENC_TABLE_64[i][j])
		}
	}

	intermediate[15] += intermediate[16] / r1Div
	intermediate[16] %= r1Div

	for i := 8; i < 16; i++ {
		for j := 0; j < 17; j++ {
			intermediate[j+1] += uint64(bin[i]) * uint64(ENC_TABLE_64[i][j])
		}
	}

	/* Now we make sure each term is less than 58^5. Again, we have to be
	   a bit careful of overflow.

	   For N==32, in the worst case, as before, intermediate[8] will be
	   just over 2^63 and intermediate[7] will be just over 2^62.6.  In
	   the first step, we'll add floor(intermediate[8]/58^5) to
	   intermediate[7].  58^5 is pretty big though, so intermediate[7]
	   barely budges, and this is still fine.

	   For N==64, in the worst case, the biggest entry in intermediate at
	   this point is 2^63.87, and in the worst case, we add (2^64-1)/58^5,
	   which is still about 2^63.87. */
	for i := 17; i > 0; i-- {
		intermediate[i-1] += (intermediate[i] / r1Div)
		intermediate[i] %= r1Div
	}

	/* Convert intermediate form to base 58.  This form of conversion
	   exposes tons of ILP, but it's more than the CPU can take advantage
	   of.
	     X = sum_i raw_base58[i] * 58^(RAW58_SZ-1-i) */
	var rawBase58 [90]byte
	for i := 0; i < 18; i++ {
		/* We know intermediate[ i ] < 58^5 < 2^32 for all i, so casting to
		   a uint is safe.  GCC doesn't seem to be able to realize this, so
		   when it converts ulong/ulong to a magic multiplication, it
		   generates the single-op 64b x 64b -> 128b mul instruction.  This
		   hurts the CPU's ability to take advantage of the ILP here. */
		v := uint(intermediate[i])
		rawBase58[5*i+4] = byte((v / 1) % 58)
		rawBase58[5*i+3] = byte((v / 58) % 58)
		rawBase58[5*i+2] = byte((v / 3364) % 58)
		rawBase58[5*i+1] = byte((v / 195112) % 58)
		rawBase58[5*i+0] = byte(v / 11316496)
	}

	/* Finally, actually convert to the string.  We have to ignore all the
	   leading zeros in raw_base58 and instead insert in_leading_0s
	   leading '1' characters.  We can show that raw_base58 actually has
	   at least in_leading_0s, so we'll do this by skipping the first few
	   leading zeros in raw_base58. */
	rawLeadingZeros := 0
	for rawLeadingZeros = 0; rawLeadingZeros < 90; rawLeadingZeros++ {
		if rawBase58[rawLeadingZeros] != 0 {
			break
		}
	}

	/* It's not immediately obvious that raw_leading_0s >= in_leading_0s,
	   but it's true.  In base b, X has floor(log_b X)+1 digits.  That
	   means in_leading_0s = N-1-floor(log_256 X) and raw_leading_0s =
	   RAW58_SZ-1-floor(log_58 X).  Let X<256^N be given and consider:

	   raw_leading_0s - in_leading_0s =
	     =  RAW58_SZ-N + floor( log_256 X ) - floor( log_58 X )
	     >= RAW58_SZ-N - 1 + ( log_256 X - log_58 X ) .

	   log_256 X - log_58 X is monotonically decreasing for X>0, so it
	   achieves it minimum at the maximum possible value for X, i.e.
	   256^N-1.
	     >= RAW58_SZ-N-1 + log_256(256^N-1) - log_58(256^N-1)

	   When N==32, RAW58_SZ is 45, so this gives skip >= 0.29
	   When N==64, RAW58_SZ is 90, so this gives skip >= 1.59.

	   Regardless, raw_leading_0s - in_leading_0s >= 0. */
	var out [88]byte
	skip := rawLeadingZeros - inLeadingZeros
	for i := 0; i < 90-skip; i++ {
		out[i] = BASE58_ALPHABET[rawBase58[skip+i]]
	}

	return string(out[:90-skip])
}

func Decode32(encoded string) ([32]byte, error) {
	if len(encoded) > BASE58_ENCODED_32_LEN {
		return [32]byte{}, errors.New("invalid size for encoded string")
	}

	for i := 0; i < len(encoded); i++ {
		idx := uint64(encoded[i]) - uint64(BASE58_INVERSE_TABLE_OFFSET)
		idx = min(idx, BASE58_INVERSE_TABLE_SENTINEL)
		if BASE58_INVERSE[idx] == BASE58_INVALID_CHAR {
			return [32]byte{}, fmt.Errorf("invalid char in position: %d", i)
		}
	}

	var rawBase58 [RAWBASE58_32_SZ]byte
	/* Prepend enough 0s to make it exactly RAW58_SZ characters */
	prependZero := RAWBASE58_32_SZ - len(encoded)
	for j := 0; j < RAWBASE58_32_SZ; j++ {
		if j < prependZero {
			rawBase58[j] = 0
		} else {
			rawBase58[j] = BASE58_INVERSE[encoded[j-prependZero]-BASE58_INVERSE_TABLE_OFFSET]
		}
	}

	/* Convert to the intermediate format (base 58^5):
	   X = sum_i intermediate[i] * 58^(5*(INTERMEDIATE_SZ-1-i)) */
	var intermediate [INTERMEDIATE_32_SZ]uint64
	for i := 0; i < INTERMEDIATE_32_SZ; i++ {
		intermediate[i] = uint64(rawBase58[5*i+0])*11316496 +
			uint64(rawBase58[5*i+1])*195112 +
			uint64(rawBase58[5*i+2])*3364 +
			uint64(rawBase58[5*i+3])*58 +
			uint64(rawBase58[5*i+4])*1
	}

	var bin [BINARY_32_SZ]uint64
	for j := 0; j < BINARY_32_SZ; j++ {
		var acc uint64 = 0
		for i := 0; i < INTERMEDIATE_32_SZ; i++ {
			acc += intermediate[i] * uint64(DEC_TABLE_32[i][j])
		}

		bin[j] = acc
	}

	/* Make sure each term is less than 2^32.

	   For N==32, we have plenty of headroom in binary, so overflow is
	   not a concern this time.

	   For N==64, even if we add 2^32 to binary[13], it is still 2^63.998,
	   so this won't overflow. */

	for i := BINARY_32_SZ - 1; i > 0; i-- {
		bin[i-1] += (bin[i] >> 32)
		bin[i] &= 0xFFFFFFFF
	}

	/* If the largest term is 2^32 or bigger, it means N is larger than
	   what can fit in BYTE_CNT bytes.  This can be triggered, by passing
	   a base58 string of all 'z's for example. */

	if bin[0] > 0xFFFFFFFF {
		return [32]byte{}, errors.New("invalid encoded parameter")
	}

	var out [32]byte
	/* Convert each term to big endian for the final output */
	for i := 0; i < BINARY_32_SZ; i++ {
		binary.BigEndian.PutUint32(out[i*4:], uint32(bin[i]))
	}

	/* Make sure the encoded version has the same number of leading '1's
	   as the decoded version has leading 0s. The check doesn't read past
	   the end of encoded, because '\0' != '1', so it will return NULL. */
	var leadingZeroCnt uint64 = 0
	for ; leadingZeroCnt < 32; leadingZeroCnt++ {
		if out[leadingZeroCnt] != 0 {
			break
		}
		if encoded[leadingZeroCnt] != '1' {
			return [32]byte{}, errors.New("counter for leading 1 failed")
		}
	}

	if encoded[leadingZeroCnt] == '1' {
		return [32]byte{}, errors.New("counter for leading 1 failed")
	}

	return out, nil
}

func Decode64(encoded string) ([64]byte, error) {
	if len(encoded) > BASE58_ENCODED_64_LEN {
		return [64]byte{}, errors.New("invalid size for encoded string")
	}

	for i := 0; i < len(encoded); i++ {
		idx := uint64(encoded[i]) - uint64(BASE58_INVERSE_TABLE_OFFSET)
		idx = min(idx, BASE58_INVERSE_TABLE_SENTINEL)
		if BASE58_INVERSE[idx] == BASE58_INVALID_CHAR {
			return [64]byte{}, fmt.Errorf("invalid char in position: %d", i)
		}
	}

	var rawBase58 [RAWBASE58_64_SZ]byte
	/* Prepend enough 0s to make it exactly RAW58_SZ characters */
	prependZero := RAWBASE58_64_SZ - len(encoded)
	for j := 0; j < RAWBASE58_64_SZ; j++ {
		if j < prependZero {
			rawBase58[j] = 0
		} else {
			rawBase58[j] = BASE58_INVERSE[encoded[j-prependZero]-BASE58_INVERSE_TABLE_OFFSET]
		}
	}

	/* Convert to the intermediate format (base 58^5):
	   X = sum_i intermediate[i] * 58^(5*(INTERMEDIATE_SZ-1-i)) */
	var intermediate [INTERMEDIATE_64_SZ]uint64
	for i := 0; i < INTERMEDIATE_64_SZ; i++ {
		intermediate[i] = uint64(rawBase58[5*i+0])*11316496 +
			uint64(rawBase58[5*i+1])*195112 +
			uint64(rawBase58[5*i+2])*3364 +
			uint64(rawBase58[5*i+3])*58 +
			uint64(rawBase58[5*i+4])*1
	}

	var bin [BINARY_64_SZ]uint64
	for j := 0; j < BINARY_64_SZ; j++ {
		var acc uint64 = 0
		for i := 0; i < INTERMEDIATE_64_SZ; i++ {
			acc += intermediate[i] * uint64(DEC_TABLE_64[i][j])
		}

		bin[j] = acc
	}

	/* Make sure each term is less than 2^32.

	   For N==32, we have plenty of headroom in binary, so overflow is
	   not a concern this time.

	   For N==64, even if we add 2^32 to binary[13], it is still 2^63.998,
	   so this won't overflow. */

	for i := BINARY_64_SZ - 1; i > 0; i-- {
		bin[i-1] += (bin[i] >> 32)
		bin[i] &= 0xFFFFFFFF
	}

	/* If the largest term is 2^32 or bigger, it means N is larger than
	   what can fit in BYTE_CNT bytes.  This can be triggered, by passing
	   a base58 string of all 'z's for example. */

	if bin[0] > 0xFFFFFFFF {
		return [64]byte{}, errors.New("invalid encoded parameter")
	}

	var out [64]byte
	/* Convert each term to big endian for the final output */
	for i := 0; i < BINARY_64_SZ; i++ {
		binary.BigEndian.PutUint32(out[i*4:], uint32(bin[i]))
	}

	/* Make sure the encoded version has the same number of leading '1's
	   as the decoded version has leading 0s. The check doesn't read past
	   the end of encoded, because '\0' != '1', so it will return NULL. */
	var leadingZeroCnt uint64 = 0
	for ; leadingZeroCnt < 64; leadingZeroCnt++ {
		if out[leadingZeroCnt] != 0 {
			break
		}
		if encoded[leadingZeroCnt] != '1' {
			return [64]byte{}, errors.New("counter for leading 1 failed")
		}
	}

	if encoded[leadingZeroCnt] == '1' {
		return [64]byte{}, errors.New("counter for leading 1 failed")
	}

	return out, nil
}
