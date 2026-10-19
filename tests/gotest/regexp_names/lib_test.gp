package lib

import "testing"

func TestA(t *testing.T)      {}
func TestAB(t *testing.T)     {}
func TestABB(t *testing.T)    {}
func TestUber(t *testing.T)   {}
func TestÜber(t *testing.T)   {}
func TestÀB(t *testing.T)     {}
func Test_x(t *testing.T)     {}
func TestFooBar(t *testing.T) {}
func TestX1(t *testing.T)     {}
func TestXy(t *testing.T)     {}
func TestA1B(t *testing.T)    {}
func BenchmarkSort(b *testing.B) {
	for i := 0; i < b.N; i++ {
	}
}
