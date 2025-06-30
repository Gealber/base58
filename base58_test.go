package base58

import (
	"crypto/rand"
	"reflect"
	"testing"
)

func Test_base58Encode32(t *testing.T) {
	data := [32]byte{225, 1, 118, 57, 196, 60, 117, 207, 131, 118, 39, 114, 43, 183, 110, 103, 209, 162, 104, 207, 202, 190, 194, 24, 165, 53, 95, 24, 51, 245, 133, 119}
	if Encode32(data) != "G9L2SYsfcuFt3eQKHQwG7JVNztdv1bwLaPFAnm1v3Pre" {
		t.Fail()
	}

	out, err := Decode32(Encode32(data))
	if err != nil {
		t.Fatalf("invalid decoding: %s", err)
	}

	if !reflect.DeepEqual(out, data) {
		t.Fatalf("got: %+v expected: %+v", out, data)
	}

	for i := 0; i < 32; i++ {
		data[i] = 0
	}

	result := Encode32(data)
	if result != "11111111111111111111111111111111" {
		t.Fatalf("result: %s", result)
	}
}

func Test_base58Encode64(t *testing.T) {
	data := [64]byte{28, 123, 237, 119, 222, 241, 46, 210, 51, 192, 180, 7, 20, 50, 209, 32, 208, 94, 170, 188, 192, 98, 202, 12, 14, 242, 63, 118, 142, 225, 147, 147, 174, 253, 6, 142, 12, 172, 66, 207, 254, 29, 84, 35, 22, 161, 190, 154, 109, 12, 191, 23, 95, 120, 140, 44, 51, 57, 123, 40, 61, 186, 225, 5}
	if Encode64(data) != "a2kzMdVRfi2Q6oGKFC3ewWdpZfmGwNACGh3HJK4hsp8DeENS7wz4ZiwM4nJ4xr21EwVoa2TtHE87if7Paiv1aha" {
		t.Fail()
	}
	out, err := Decode64(Encode64(data))
	if err != nil {
		t.Fail()
	}

	if !reflect.DeepEqual(out, data) {
		t.Fail()
	}
}

func Benchmark_base58Encode32(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var data [32]byte
		rand.Read(data[:])
		Encode32(data)
	}
}

func Benchmark_base58Encode64(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var data [64]byte
		rand.Read(data[:])
		Encode64(data)
	}
}

func Benchmark_base58Decode32(b *testing.B) {
	encoded := "G9L2SYsfcuFt3eQKHQwG7JVNztdv1bwLaPFAnm1v3Pre"
	for i := 0; i < b.N; i++ {
		Decode32(encoded)
	}
}

func Benchmark_base58Decode64(b *testing.B) {
	encoded := "G9L2SYsfcuFt3eQKHQwG7JVNztdv1bwLaPFAnm1v3Pre"
	for i := 0; i < b.N; i++ {
		Decode64(encoded)
	}
}

// BENCHMARKS WITH MTRON lib uncomment to run them
// func Benchmark_mtronB58Decode_64(b *testing.B) {
// 	encoded := "a2kzMdVRfi2Q6oGKFC3ewWdpZfmGwNACGh3HJK4hsp8DeENS7wz4ZiwM4nJ4xr21EwVoa2TtHE87if7Paiv1aha"
// 	for i := 0; i < b.N; i++ {
// 		base58.FastBase58Decoding(encoded)
// 	}
// }
//
// func Benchmark_mtronB58_64(b *testing.B) {
// 	for i := 0; i < b.N; i++ {
// 		var data [64]byte
// 		rand.Read(data[:])
// 		base58.FastBase58Encoding(data[:])
// 	}
// }
//
// func Benchmark_mtronB58Decode_32(b *testing.B) {
// 	encoded := "G9L2SYsfcuFt3eQKHQwG7JVNztdv1bwLaPFAnm1v3Pre"
// 	for i := 0; i < b.N; i++ {
// 		base58.FastBase58Decoding(encoded)
// 	}
// }
//
// func Benchmark_mtronB58_32(b *testing.B) {
// 	for i := 0; i < b.N; i++ {
// 		var data [32]byte
// 		rand.Read(data[:])
// 		base58.FastBase58Encoding(data[:])
// 	}
// }
